package gotdx

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// fakeRespHeader 按 `proto.RespHeader` 的字段顺序构造 16 字节响应头，供本地模拟服务器回包使用。
func fakeRespHeader(seq uint32, method uint16, size uint16) []byte {
	b := make([]byte, proto.MessageHeaderBytes)
	binary.LittleEndian.PutUint32(b[0:4], 0) // 保留标识字节（对应 `I1`）
	b[4] = 0                                 // 保留标识字节（对应 `I2`）
	binary.LittleEndian.PutUint32(b[5:9], seq)
	b[9] = 0 // 保留标识字节（对应 `I3`）
	binary.LittleEndian.PutUint16(b[10:12], method)
	binary.LittleEndian.PutUint16(b[12:14], size)
	binary.LittleEndian.PutUint16(b[14:16], size)
	return b
}

// fakeHello1Body 构造不少于 189 字节的 `Hello1` 握手正文。
func fakeHello1Body() []byte {
	body := make([]byte, 189)
	binary.LittleEndian.PutUint16(body[1:3], 2026)
	body[3] = 11
	body[4] = 8
	copy(body[67:89], "fake-server-name")
	copy(body[89:153], "http://fake.example.com")
	copy(body[159:189], "fake-category")
	return body
}

// fakeTDXServer 模拟通达信主站：握手正常；对 `KMSG_HISTORYMINUTETIMEDATE` 命令第一次回超长响应，
// 之后按正常报文应答。返回本地监听地址，第二个返回值为关闭监听的回调函数。
func fakeTDXServer(t *testing.T, oversizedSize uint16) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				tickCount := 0
				for {
					header := make([]byte, 12)
					if _, err := io.ReadFull(conn, header); err != nil {
						return
					}
					seq := binary.LittleEndian.Uint32(header[1:5])
					pkgLen := binary.LittleEndian.Uint16(header[8:10])
					method := binary.LittleEndian.Uint16(header[10:12])
					payloadLen := 0
					if pkgLen > 2 {
						payloadLen = int(pkgLen - 2)
					}
					if _, err := io.ReadFull(conn, make([]byte, payloadLen)); err != nil {
						return
					}
					switch method {
					case proto.KMSG_CMD1:
						body := fakeHello1Body()
						conn.Write(append(fakeRespHeader(seq, method, uint16(len(body))), body...))
					case proto.KMSG_HISTORYMINUTETIMEDATE:
						tickCount++
						if tickCount == 1 && oversizedSize > 0 {
							body := make([]byte, oversizedSize)
							for i := range body {
								body[i] = 0xAA
							}
							conn.Write(append(fakeRespHeader(seq, method, oversizedSize), body...))
						} else {
							// 正常响应：10 字节负载（2 字节返回条数加 2 个语义待确认的 `uint32` 字段）。
							conn.Write(append(fakeRespHeader(seq, method, 10), make([]byte, 10)...))
						}
					default:
						conn.Write(fakeRespHeader(seq, method, 0))
					}
				}
			}(conn)
		}
	}()

	return ln.Addr().String(), func() { _ = ln.Close() }
}

// 验证超长响应返回 `ErrBadData` 后连接被丢弃，避免残留字节污染后续请求。
func TestExchangeBadDataDiscardsConn(t *testing.T) {
	addr, stop := fakeTDXServer(t, 59202)
	defer stop()

	c := New(WithTCPAddress(addr), WithTimeoutSec(5))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	if c.conn == nil {
		t.Fatal("expected connected client")
	}

	if _, err := c.StockHistoryTickChart(20260811, 0, "000779"); err != types.ErrBadData {
		t.Fatalf("expected ErrBadData, got %v", err)
	}
	if c.conn != nil {
		t.Fatal("connection must be discarded after ErrBadData")
	}
}

// 验证正常响应不会丢弃连接。
func TestExchangeOKKeepsConn(t *testing.T) {
	addr, stop := fakeTDXServer(t, 0)
	defer stop()

	c := New(WithTCPAddress(addr), WithTimeoutSec(5))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer c.Disconnect()

	if _, err := c.StockHistoryTickChart(20260811, 0, "000779"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.conn == nil {
		t.Fatal("connection must be kept on success")
	}
	// 连续第二次请求仍可用，验证连接未被误丢弃。
	if _, err := c.StockHistoryTickChart(20260811, 0, "000779"); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
}

// 验证写读超时后连接被丢弃，避免半包残留污染后续请求。
func TestExchangeTimeoutDiscardsConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				for {
					header := make([]byte, 12)
					if _, err := io.ReadFull(conn, header); err != nil {
						return
					}
					seq := binary.LittleEndian.Uint32(header[1:5])
					pkgLen := binary.LittleEndian.Uint16(header[8:10])
					method := binary.LittleEndian.Uint16(header[10:12])
					payloadLen := 0
					if pkgLen > 2 {
						payloadLen = int(pkgLen - 2)
					}
					if _, err := io.ReadFull(conn, make([]byte, payloadLen)); err != nil {
						return
					}
					if method == proto.KMSG_CMD1 {
						body := fakeHello1Body()
						conn.Write(append(fakeRespHeader(seq, method, uint16(len(body))), body...))
						continue
					}
					// 分时请求：只发响应头前 10 字节后挂起，触发客户端读超时。
					full := append(fakeRespHeader(seq, method, 10), make([]byte, 10)...)
					conn.Write(full[:10])
					time.Sleep(3 * time.Second)
					return
				}
			}(conn)
		}
	}()

	c := New(WithTCPAddress(ln.Addr().String()), WithTimeoutSec(1))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	start := time.Now()
	if _, err := c.StockHistoryTickChart(20260811, 0, "000779"); err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
		t.Fatalf("expected quick timeout, took %v", elapsed)
	}
	if c.conn != nil {
		t.Fatal("connection must be discarded after read timeout")
	}
}
