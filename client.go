package gotdx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

type clientMode uint8

const (
	clientModeMain clientMode = iota
	clientModeEx
	clientModeMacMain
	clientModeMacEx
)

// New 创建连接通达信主行情服务器的客户端。
//
// 参数 `opts` 为按顺序覆盖内置默认配置的函数式选项，后传入的选项优先生效；不传选项时使用内置
// 主站地址池、默认超时与重试次数。返回的 `Client` 零值不可用，须由 `New` 系列构造；其方法可
// 并发调用，同一连接上的请求由客户端内部串行化。
func New(opts ...Option) *Client {
	return newClientWithOptions(applyOptions(opts...), clientModeMain)
}

// NewEx 创建连接通达信扩展行情服务器（港股、美股、期货等）的客户端。
//
// 参数 `opts` 为按顺序覆盖内置默认配置的函数式选项，后传入的选项优先生效；不传选项时使用内置
// 扩展站地址池与默认超时。返回的 `Client` 零值不可用，方法可并发调用。
func NewEx(opts ...Option) *Client {
	return newClientWithOptions(applyOptions(opts...), clientModeEx)
}

// NewMAC 创建连接 MAC 行情主站的客户端。
//
// 参数 `opts` 为按顺序覆盖内置默认配置的函数式选项，后传入的选项优先生效；不传选项时使用内置
// MAC 主站地址池与默认超时。返回的 `Client` 零值不可用，方法可并发调用。
func NewMAC(opts ...Option) *Client {
	return newClientWithOptions(applyOptions(opts...), clientModeMacMain)
}

// NewMACEx 创建连接 MAC 扩展行情服务器的客户端。
//
// 参数 `opts` 为按顺序覆盖内置默认配置的函数式选项，后传入的选项优先生效；不传选项时使用内置
// MAC 扩展站地址池与默认超时。返回的 `Client` 零值不可用，方法可并发调用。
func NewMACEx(opts ...Option) *Client {
	return newClientWithOptions(applyOptions(opts...), clientModeMacEx)
}

// Client 是通达信行情协议客户端，封装 TCP 连接、握手、协议收发以及主站、扩展站、MAC 站的高层接口。
//
// Client 须由 `New`、`NewEx`、`NewMAC`、`NewMACEx` 构造，零值不可用；各方法可并发调用，同一连接上的
// 请求由客户端内部串行化，调用方无需自行加锁。
type Client struct {
	conn                net.Conn
	opt                 *Options
	complete            chan bool
	sending             chan bool
	mu                  sync.Mutex
	mode                clientMode
	main                *Client
	ex                  *Client
	decimalPoints       map[uint8]map[string]int8
	decimalPointsLoaded map[uint8]bool
}

// CurrentAddress 返回当前配置（或最近连接成功）的服务器 `host:port` 地址。
//
// 本方法无业务入参；客户端指针或内部选项为空时返回空字符串；已拨号成功后返回实际连通的地址，
// 未拨号时返回配置的单点地址。
func (client *Client) CurrentAddress() string {
	if client == nil || client.opt == nil {
		return ""
	}
	return client.opt.TCPAddress
}

// ProbeHosts 并发探测客户端已配置的全部地址，返回按可达性优先、同可达时延迟从低到高排序的
// 探测结果；每条结果含地址、可达标志与拨号往返耗时，不可达条目同时携带失败原因。
//
// 本方法无业务入参；客户端指针或内部选项为空时返回 `nil`；探测使用当前配置的连接超时。
func (client *Client) ProbeHosts() []HostProbeResult {
	if client == nil || client.opt == nil {
		return nil
	}
	return ProbeAddresses(client.addresses(), client.timeout())
}

// FastestHost 返回已配置地址中延迟最低且可达的一条探测结果。
//
// 本方法无业务入参；全部地址不可达，或客户端指针、内部选项为空时返回零值 `HostProbeResult` 与
// `ErrNoReachableHosts` 错误；探测使用当前配置的连接超时。
func (client *Client) FastestHost() (HostProbeResult, error) {
	if client == nil || client.opt == nil {
		return HostProbeResult{}, ErrNoReachableHosts
	}
	return FastestAddress(client.addresses(), client.timeout())
}

// newClientWithOptions 基于选项与站点模式构造客户端。
//
// 参数 `opt` 为待深拷贝的配置，空指针时回退到默认配置；参数 `mode` 标识主站、扩展站、MAC 主站或
// MAC 扩展站，不同模式把对应站点的单点地址与地址池切换为后续拨号使用的默认目标。
func newClientWithOptions(opt *Options, mode clientMode) *Client {
	client := &Client{
		opt:      cloneOptions(opt),
		sending:  make(chan bool, 1),
		complete: make(chan bool, 1),
		mode:     mode,
	}

	// 扩展站 / MAC 站点模式：用对应站点的地址与地址池覆盖主站连接配置。
	if mode == clientModeEx {
		client.opt.TCPAddress = client.opt.ExTCPAddress
		client.opt.TCPAddressPool = append([]string(nil), client.opt.ExTCPAddressPool...)
	}
	if mode == clientModeMacMain {
		client.opt.TCPAddress = client.opt.MacTCPAddress
		client.opt.TCPAddressPool = append([]string(nil), client.opt.MacTCPAddressPool...)
	}
	if mode == clientModeMacEx {
		client.opt.TCPAddress = client.opt.MacExTCPAddress
		client.opt.TCPAddressPool = append([]string(nil), client.opt.MacExTCPAddressPool...)
	}

	return client
}

// cloneOptions 深拷贝 `Options`。
//
// 参数 `opt` 为空时返回默认配置；结构体值复制后，四个地址池单独分配切片，避免不同客户端的连接
// 配置共享底层切片。
func cloneOptions(opt *Options) *Options {
	if opt == nil {
		return defaultOptions()
	}
	clone := *opt
	clone.TCPAddressPool = append([]string(nil), opt.TCPAddressPool...)
	clone.ExTCPAddressPool = append([]string(nil), opt.ExTCPAddressPool...)
	clone.MacTCPAddressPool = append([]string(nil), opt.MacTCPAddressPool...)
	clone.MacExTCPAddressPool = append([]string(nil), opt.MacExTCPAddressPool...)
	return &clone
}

// connect 按连接顺序逐个尝试地址，任一地址建立 TCP 连接即返回 `nil`。
//
// 候选地址为空时返回"未配置 TCP 地址"错误；全部地址拨号失败时返回最后一次拨号错误。
func (client *Client) connect() error {
	addresses := client.connectionOrder()
	if len(addresses) == 0 {
		return errors.New("no tcp address configured")
	}

	var lastErr error
	for _, address := range addresses {
		// 单个地址拨号失败不中断流程，记录错误后继续尝试下一个备选地址。
		if err := client.connectToAddress(address); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	// 兜底错误：循环中未产生具体错误（理论上不会走到该分支）。
	if lastErr == nil {
		lastErr = errors.New("no available tcp address")
	}
	return lastErr
}

// connectToAddress 使用当前超时拨号单个地址。
//
// 参数 `address` 为 `host:port` 形式地址；拨号失败返回底层 `net.DialTimeout` 错误，成功后把实际
// 连通的地址写回选项，供 `CurrentAddress` 回读。
func (client *Client) connectToAddress(address string) error {
	conn, err := net.DialTimeout("tcp", address, client.timeout())
	if err != nil {
		return err
	}
	client.conn = conn
	client.opt.TCPAddress = address
	return nil
}

// addresses 汇总单点地址与地址池，返回 `host:port` 地址序列；单点地址排在前面优先尝试，地址池
// 按配置顺序跟随其后。
func (client *Client) addresses() []string {
	addresses := make([]string, 0, 1+len(client.opt.TCPAddressPool))
	if client.opt.TCPAddress != "" {
		addresses = append(addresses, client.opt.TCPAddress)
	}
	addresses = append(addresses, client.opt.TCPAddressPool...)
	return addresses
}

// connectionOrder 决定本次拨号的地址顺序。
//
// 未开启测速时保持配置原顺序；开启时先按可达性与延迟排序，不可达与重复地址去重后，再把测速未
// 覆盖的原始地址补在末尾。客户端或选项为空、候选集合为空时返回 `nil`；测速全部失败时回退到原始
// 地址顺序。
func (client *Client) connectionOrder() []string {
	if client == nil || client.opt == nil {
		return nil
	}
	addresses := client.addresses()
	if len(addresses) == 0 {
		return nil
	}
	if !client.opt.AutoSelectFastest {
		return addresses
	}

	// 测速全部失败时回退到原始地址顺序，避免直接中断连接。
	results := ProbeAddresses(addresses, client.timeout())
	if len(results) == 0 {
		return addresses
	}

	ordered := make([]string, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, result := range results {
		if !result.Reachable {
			continue
		}
		if _, ok := seen[result.Address]; ok {
			continue
		}
		ordered = append(ordered, result.Address)
		seen[result.Address] = struct{}{}
	}
	// 再补齐测速结果中未出现的原始地址，保证候选集合不丢失。
	for _, address := range addresses {
		if _, ok := seen[address]; ok {
			continue
		}
		ordered = append(ordered, address)
	}
	return ordered
}

// timeout 返回本次连接与读写共用的超时时长。
//
// 客户端、选项缺失，或 `Options.TimeoutSec` 为非正数时回退到内置默认超时秒数。
func (client *Client) timeout() time.Duration {
	if client == nil || client.opt == nil {
		return time.Duration(_defaultTimeoutSec) * time.Second
	}
	timeout := time.Duration(client.opt.TimeoutSec) * time.Second
	if timeout <= 0 {
		return time.Duration(_defaultTimeoutSec) * time.Second
	}
	return timeout
}

// closeCurrentConn 关闭并清空当前连接；关闭错误忽略，连接置空后便于后续重建。
func (client *Client) closeCurrentConn() {
	if client.conn != nil {
		_ = client.conn.Close()
		client.conn = nil
	}
}

// connectWithHandshake 逐个地址拨号并执行握手回调。
//
// 参数 `handshake` 为在已拨号连接上执行的握手函数，其返回 `nil` 表示握手成功；握手失败说明该
// 连接帧状态不可信，立即关闭后切换下一个地址重试。候选地址为空返回"未配置 TCP 地址"错误；全部
// 地址拨号或握手失败时返回最后一次错误。
func (client *Client) connectWithHandshake(handshake func() error) error {
	addresses := client.connectionOrder()
	if len(addresses) == 0 {
		return errors.New("no tcp address configured")
	}

	var lastErr error
	for _, address := range addresses {
		if err := client.connectToAddress(address); err != nil {
			lastErr = err
			continue
		}
		if err := handshake(); err == nil {
			return nil
		} else {
			// 握手失败的连接帧状态不可信，关闭后再尝试下一个地址。
			lastErr = err
			client.closeCurrentConn()
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no available tcp address")
	}
	return lastErr
}

// exchange 完成一次请求-响应收发：编码请求、带超时写出发送、按帧读取响应头与负载，压缩帧按需
// 以 `zlib` 解压。
//
// 参数 `builder` 负责协议请求的字节编码；返回的 `respHeader` 为解码后的响应头，`payload`
// 为解压后的明文负载。尚未建立连接、请求编码失败、写重试超限、帧头或负载读取失败、负载长度超过
// `MessageMaxBytes`、解压失败等任一环节出错都会返回错误，并丢弃当前连接，避免残留字节破坏后续
// 帧边界。
func (client *Client) exchange(builder proto.RequestBuilder) (respHeader *proto.RespHeader, payload []byte, err error) {
	// 未建立连接时直接报错，避免对空连接执行读写。
	if client.conn == nil {
		return nil, nil, errors.New("connection is nil")
	}

	sendData, err := builder.BuildRequest()
	if err != nil {
		return nil, nil, err
	}

	// 写读任一环节失败后，连接上的字节流无法保证仍处于帧边界，
	// 必须丢弃连接，避免残留字节被后续请求误读为响应头。
	defer func() {
		if err != nil {
			client.closeCurrentConn()
		}
	}()

	// 为本次收发设置整体截止时间，结束后清空超时以免影响后续操作。
	_ = client.conn.SetDeadline(time.Now().Add(client.timeout()))
	defer func() {
		_ = client.conn.SetDeadline(time.Time{})
	}()

	retryTimes := 0

	for {
		n, err := client.conn.Write(sendData)
		// 发送不完整时按 `MaxRetryTimes` 重试；超过重试次数或写入出错则放弃本次请求。
		if n < len(sendData) {
			retryTimes++
			if retryTimes <= client.opt.MaxRetryTimes {
				log.Printf("第%d次重试\n", retryTimes)
			} else {
				return nil, nil, err
			}
		} else {
			if err != nil {
				return nil, nil, err
			}
			break
		}
	}

	// 按固定帧长读取响应头，网络不足一帧时 `io.ReadFull` 会等待至凑满或报错。
	headerBytes := make([]byte, proto.MessageHeaderBytes)
	_, err = io.ReadFull(client.conn, headerBytes)
	if err != nil {
		return nil, nil, err
	}

	// 响应头解码。
	headerBuf := bytes.NewReader(headerBytes)
	var header proto.RespHeader
	if err := binary.Read(headerBuf, binary.LittleEndian, &header); err != nil {
		return nil, nil, err
	}

	// 负载长度超出协议上限视为非法数据，防止畸形长度触发过大的内存分配。
	if header.ZipSize > proto.MessageMaxBytes {
		log.Printf("msgData has bytes(%d) beyond max %d\n", header.ZipSize, proto.MessageMaxBytes)
		return nil, nil, types.ErrBadData
	}

	msgData := make([]byte, header.ZipSize)
	_, err = io.ReadFull(client.conn, msgData)
	if err != nil {
		return nil, nil, err
	}

	// 压缩大小与解压大小不一致表示负载经过 `zlib` 压缩，否则直接返回明文负载。
	if header.ZipSize != header.UnZipSize {
		var out bytes.Buffer
		b := bytes.NewReader(msgData)
		r, err := zlib.NewReader(b)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()
		if _, err := io.Copy(&out, r); err != nil {
			return nil, nil, err
		}
		return &header, out.Bytes(), nil
	}

	return &header, msgData, nil
}

// Connect 连接通达信主行情服务器并完成 `Hello1` 握手。
//
// 本方法无业务入参；按候选地址顺序逐个拨号，握手成功后返回主站握手应答 `*proto.Hello1Reply`；全部
// 候选地址拨号或握手失败时返回最后一次错误，应答为 `nil`。扩展站客户端会复用内嵌主站连接完成本次
// 握手。
func (client *Client) Connect() (*proto.Hello1Reply, error) {
	// 扩展站客户端复用内嵌主站连接完成主站握手。
	if client.mode == clientModeEx {
		client.mu.Lock()
		if client.main == nil {
			client.main = newClientWithOptions(client.opt, clientModeMain)
		}
		main := client.main
		client.mu.Unlock()
		return main.Connect()
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	obj := proto.NewHello1()
	var reply *proto.Hello1Reply
	// 握手回调内发送 `Hello1` 请求并解析应答，失败由外层切换下一地址重试。
	err := client.connectWithHandshake(func() error {
		var err error
		reply, err = client.executeLocked(obj)
		return err
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// ConnectEx 连接扩展市场服务器并完成登录握手。
//
// 本方法无业务入参；按候选地址顺序逐个拨号，登录成功后返回扩展站登录应答 `*proto.ExLoginReply`；
// 全部候选地址拨号或登录失败时返回最后一次错误，应答为 `nil`。主站客户端会复用内嵌扩展站连接完成
// 本次登录。
func (client *Client) ConnectEx() (*proto.ExLoginReply, error) {
	// 主站客户端复用内嵌扩展站连接完成扩展站登录。
	if client.mode == clientModeMain {
		client.mu.Lock()
		if client.ex == nil {
			client.ex = newClientWithOptions(client.opt, clientModeEx)
		}
		ex := client.ex
		client.mu.Unlock()
		return ex.ConnectEx()
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	obj := proto.NewExLogin()
	var reply *proto.ExLoginReply
	err := client.connectWithHandshake(func() error {
		var err error
		reply, err = client.executeLocked(obj)
		return err
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// Disconnect 断开当前连接，并级联关闭关联的内嵌主站、扩展站连接。
//
// 本方法无业务入参；返回关闭过程中遇到的首个错误，后续内嵌连接的关闭错误不会覆盖先出现的错误；
// 已关闭的内嵌连接会跳过重复关闭。
func (client *Client) Disconnect() error {
	client.mu.Lock()
	conn := client.conn
	client.conn = nil
	main := client.main
	ex := client.ex
	client.main = nil
	client.ex = nil
	client.mu.Unlock()

	var err error
	if conn != nil {
		err = conn.Close()
	}
	// 内嵌主站可能就是自身，跳过避免重复关闭；错误不覆盖主连接错误。
	if main != nil && main != client {
		if closeErr := main.Disconnect(); err == nil {
			err = closeErr
		}
	}
	// 扩展站与主站为不同连接时再级联关闭。
	if ex != nil && ex != client && ex != main {
		if closeErr := ex.Disconnect(); err == nil {
			err = closeErr
		}
	}
	return err
}
