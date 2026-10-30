package gotdx

import (
	"errors"
	"io"
	"net"
	"testing"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// executeGenericMethodProtocol 是 `execute` 泛型流程的最小协议桩：请求体长度固定为 1 字节，
// 响应负载原样保存为应答字符串，用于在 `net.Pipe` 上走通一次收发流程。
type executeGenericMethodProtocol struct {
	reply string // 最近一次响应负载转成的应答字符串，供 `Response` 返回
}

// BuildRequest 返回固定的请求负载 `0x5a`；不携带业务参数，编码恒成功，错误返回 `nil`。
func (*executeGenericMethodProtocol) BuildRequest() ([]byte, error) {
	return []byte{0x5a}, nil
}

// ParseResponse 把响应负载原样转为字符串存入 `reply`；入参 `header` 本桩不使用，
// 负载解码恒成功，错误返回 `nil`。
func (p *executeGenericMethodProtocol) ParseResponse(_ *proto.RespHeader, payload []byte) error {
	p.reply = string(payload)
	return nil
}

// Response 返回最近一次保存的应答字符串。
func (p *executeGenericMethodProtocol) Response() string {
	return p.reply
}

func TestClientExecuteGenericMethod(t *testing.T) {
	// 使用 `net.Pipe` 在同一进程内模拟一条连接，避免依赖真实网络。
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	client := New(WithTimeoutSec(1))
	client.conn = clientConn

	serverDone := make(chan error, 1)
	go func() {
		request := make([]byte, 1)
		if _, err := io.ReadFull(serverConn, request); err != nil {
			serverDone <- err
			return
		}
		if request[0] != 0x5a {
			serverDone <- errors.New("unexpected request payload")
			return
		}

		_, err := serverConn.Write(append(fakeRespHeader(0, 0, 5), []byte("reply")...))
		serverDone <- err
	}()

	reply, err := client.execute(&executeGenericMethodProtocol{})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if reply != "reply" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("server failed: %v", err)
	}
}

func TestMakeStocks(t *testing.T) {
	stocks, err := makeStocks([]uint8{types.MarketSZ.Uint8(), types.MarketSH.Uint8()}, []string{"000001", "600000"})
	if err != nil {
		t.Fatalf("makeStocks failed: %v", err)
	}
	if len(stocks) != 2 {
		t.Fatalf("unexpected stock len: %d", len(stocks))
	}
	if stocks[0].Market != types.MarketSZ.Uint8() || stocks[0].Code != "000001" {
		t.Fatalf("unexpected first stock: %+v", stocks[0])
	}
	if stocks[1].Market != types.MarketSH.Uint8() || stocks[1].Code != "600000" {
		t.Fatalf("unexpected second stock: %+v", stocks[1])
	}
}

func TestMakeStocksCountMismatch(t *testing.T) {
	if _, err := makeStocks([]uint8{types.MarketSZ.Uint8()}, []string{"000001", "600000"}); err == nil {
		t.Fatal("expected error on count mismatch")
	}
}

func TestMakeFixedBuffers(t *testing.T) {
	code := makeCode6("600000EXTRA")
	if string(code[:]) != "600000" {
		t.Fatalf("unexpected code buffer: %q", string(code[:]))
	}

	code9 := makeCode9("TSLA-EXTRA")
	if string(code9[:4]) != "TSLA" {
		t.Fatalf("unexpected code9 buffer: %q", string(code9[:4]))
	}

	code22 := makeCode22("TSLA")
	if string(code22[:4]) != "TSLA" {
		t.Fatalf("unexpected code22 buffer: %q", string(code22[:4]))
	}

	code23 := makeCode23("09988")
	if string(code23[:5]) != "09988" {
		t.Fatalf("unexpected code23 buffer: %q", string(code23[:5]))
	}

	file40 := makeFixed40("block.dat")
	if string(file40[:9]) != "block.dat" {
		t.Fatalf("unexpected file40 buffer: %q", string(file40[:9]))
	}

	file80 := makeFixed80("test.txt")
	if string(file80[:8]) != "test.txt" {
		t.Fatalf("unexpected file80 buffer: %q", string(file80[:8]))
	}

	file300 := makeFixed300("foo")
	if string(file300[:3]) != "foo" {
		t.Fatalf("unexpected file300 buffer: %q", string(file300[:3]))
	}

	file43 := makeFixed43("TSLA")
	if string(file43[:4]) != "TSLA" {
		t.Fatalf("unexpected file43 buffer: %q", string(file43[:4]))
	}
}

func TestQuotesSortReverse(t *testing.T) {
	if got := quotesSortReverse(types.SortCode, true); got != 0 {
		t.Fatalf("unexpected code sort reverse: %d", got)
	}
	if got := quotesSortReverse(types.SortPrice, false); got != 1 {
		t.Fatalf("unexpected asc sort reverse: %d", got)
	}
	if got := quotesSortReverse(types.SortPrice, true); got != 2 {
		t.Fatalf("unexpected desc sort reverse: %d", got)
	}
}

func TestMakeExStocks(t *testing.T) {
	stocks, err := makeExStocks([]uint8{types.ExCategoryUSStock, types.ExCategoryHKMainBoard}, []string{"TSLA", "09988"})
	if err != nil {
		t.Fatalf("makeExStocks failed: %v", err)
	}
	if len(stocks) != 2 {
		t.Fatalf("unexpected stock len: %d", len(stocks))
	}
	if stocks[0].Category != types.ExCategoryUSStock || stocks[0].Code != "TSLA" {
		t.Fatalf("unexpected first ex stock: %+v", stocks[0])
	}
}
