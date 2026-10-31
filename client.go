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

// Client 是通达信行情协议客户端，封装 TCP 连接池、握手、协议收发以及主站、扩展站、MAC 站的高层接口。
//
// Client 须由 `New`、`NewEx`、`NewMAC`、`NewMACEx` 构造，零值不可用；各方法可并发调用，请求
// 按轮询分散到池内各连接：连接之间并行、单连接上的请求由该连接内部串行化，调用方无需自行加锁。
type Client struct {
	opt                 *Options      // 深拷贝后的连接配置
	pool                []*pooledConn // 连接池：每条连接独立串行锁，连接之间并行
	mu                  sync.Mutex    // 池管理锁：保护借还、补建、关闭与池状态
	closed              bool          // 池已关闭标记，关闭后拒绝借出
	next                int           // 轮询游标：借出连接时按此下标取池内连接并自增
	done                chan struct{} // 补建循环停止信号，`Disconnect` 时关闭
	refill              sync.Once     // 补建循环只启动一次
	mode                clientMode
	main                *Client
	ex                  *Client
	decimalPoints       map[uint8]map[string]int8
	decimalPointsLoaded map[uint8]bool
}

// pooledConn 为池内一条 TCP 连接：自带串行锁，保证该连接上的请求严格
// 先写后读、帧边界不被并发破坏。
type pooledConn struct {
	conn net.Conn
	mu   sync.Mutex // 该连接串行锁
	addr string     // 实际连通的 `host:port` 地址
}

// CurrentAddress 返回当前池内首条连接实际连通的服务器 `host:port` 地址。
//
// 本方法无业务入参；客户端指针或内部选项为空时返回空字符串；已建立连接池时返回首条连接的
// 实际地址，未拨号时返回配置的单点地址。
func (client *Client) CurrentAddress() string {
	if client == nil || client.opt == nil {
		return ""
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.pool) > 0 {
		return client.pool[0].addr
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
		opt:  cloneOptions(opt),
		mode: mode,
	}
	// 连接池大小非正数时回退默认，避免零值构造出不可用池。
	if client.opt.PoolSize <= 0 {
		client.opt.PoolSize = _defaultPoolSize
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

// probeOrder 并发测速并按可达性、延迟排序地址：可达且延迟低的地址排前，不可达与重复
// 地址剔除，测速结果未覆盖的原始地址补在末尾避免候选丢失；测速全部失败时按原始顺序返回。
//
// 参数 `addresses` 为待排序的 `host:port` 地址；`timeout` 为单次拨号探测超时，非正数时
// 由 `ProbeAddresses` 回退默认超时。
func probeOrder(addresses []string, timeout time.Duration) []string {
	results := ProbeAddresses(addresses, timeout)
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

// buildPoolConn 拨号指定地址并在该连接上完成当前模式握手，返回可入池连接与其握手应答。
//
// 参数 `address` 为 `host:port` 形式地址；拨号或握手失败时关闭连接并返回错误。主站与扩展站
// 各自完成对应握手协议，MAC 站拨号即视为连接成功、应答为 `nil`。
func (client *Client) buildPoolConn(address string) (*pooledConn, any, error) {
	conn, err := net.DialTimeout("tcp", address, client.timeout())
	if err != nil {
		return nil, nil, err
	}
	pc := &pooledConn{conn: conn, addr: address}
	// 握手按连接串行锁执行（此时池未并发，锁仅为形态一致）。
	pc.mu.Lock()
	reply, err := client.handshakeOn(pc)
	pc.mu.Unlock()
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return pc, reply, nil
}

// handshakeOn 在指定连接上执行当前模式对应的握手协议，返回解析后的握手应答。
//
// 主站模式发送 `Hello1` 请求并解析应答，扩展站模式发送 `ExLogin` 请求并解析应答；
// MAC 主站与 MAC 扩展站无握手协议，直接返回 `nil`。握手失败时返回对应错误。
func (client *Client) handshakeOn(pc *pooledConn) (any, error) {
	switch client.mode {
	case clientModeEx:
		obj := proto.NewExLogin()
		header, payload, err := client.exchange(pc.conn, obj)
		if err != nil {
			return nil, err
		}
		if err := obj.ParseResponse(header, payload); err != nil {
			return nil, err
		}
		return obj.Response(), nil
	case clientModeMacMain, clientModeMacEx:
		return nil, nil
	default:
		obj := proto.NewHello1()
		header, payload, err := client.exchange(pc.conn, obj)
		if err != nil {
			return nil, err
		}
		if err := obj.ParseResponse(header, payload); err != nil {
			return nil, err
		}
		return obj.Response(), nil
	}
}

// removeAddress 从地址切片中剔除指定地址，返回新切片；被剔除地址可能为
// 重复项，全部剔除后剩余切片可为空。
func removeAddress(addresses []string, target string) []string {
	out := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if addr != target {
			out = append(out, addr)
		}
	}
	return out
}

// acquire 从池内借出一条连接（轮询游标递增），调用方随后须持该连接串行锁执行请求。
//
// 池已关闭或池内暂无连接（未连接或连接全部被丢弃待补建）时返回"无可用连接"错误。
func (client *Client) acquire() (*pooledConn, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed || len(client.pool) == 0 {
		return nil, errors.New("no available connection")
	}
	idx := client.next % len(client.pool)
	client.next++
	return client.pool[idx], nil
}

// discard 关闭并移出池内指定连接；关闭错误忽略。运行期断连由此路径清理，
// 后续由补建循环补齐池容量。
func (client *Client) discard(pc *pooledConn) {
	client.mu.Lock()
	defer client.mu.Unlock()
	_ = pc.conn.Close()
	for i, p := range client.pool {
		if p == pc {
			client.pool = append(client.pool[:i], client.pool[i+1:]...)
			break
		}
	}
}

// refillLoop 按固定间隔触发补建，每分钟额外触发一次最快服务器重平衡，
// 收到停止信号（`Disconnect` 关闭 `done`）时退出。
func (client *Client) refillLoop() {
	refillTicker := time.NewTicker(time.Second)
	rebalanceTicker := time.NewTicker(time.Minute)
	defer refillTicker.Stop()
	defer rebalanceTicker.Stop()
	for {
		select {
		case <-client.done:
			return
		case <-refillTicker.C:
			client.refillPool()
		case <-rebalanceTicker.C:
			client.rebalancePool()
		}
	}
}

// rebalancePool 每分钟对候选地址重新测速一次，把连接池切换到当前最快的服务器。
//
// 先并发探活并按可达性、延迟排序（探活超时限制 5 秒），取首个可达地址为最快目标；
// 池内连接已全部指向该地址时不作调整。否则并发预建 `PoolSize` 条指向最快地址的连接，
// 全部失败保持现状；至少建成一条则先入池再关闭旧连接（先建后删，切换期间不出现空窗），
// 未建满的余量由补建循环继续补齐。
func (client *Client) rebalancePool() {
	// 探活与拨号均为网络操作，在锁外执行，避免阻塞其他调用方借还连接。
	probeTimeout := min(client.timeout(), 5*time.Second)
	ordered := probeOrder(client.addresses(), probeTimeout)
	if len(ordered) == 0 {
		return
	}
	fastest := ordered[0]

	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return
	}
	// 池内已全部指向最快地址时无需切换。
	allFastest := len(client.pool) > 0
	for _, pc := range client.pool {
		if pc.addr != fastest {
			allFastest = false
			break
		}
	}
	if allFastest {
		client.mu.Unlock()
		return
	}
	client.mu.Unlock()

	// 并发预建指向最快地址的连接，单条失败不影响其余，整体耗时由最慢一次拨号决定。
	poolSize := client.opt.PoolSize
	var (
		newMu    sync.Mutex
		newConns = make([]*pooledConn, 0, poolSize)
		wg       sync.WaitGroup
	)
	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pc, _, err := client.buildPoolConn(fastest)
			if err != nil {
				return
			}
			newMu.Lock()
			newConns = append(newConns, pc)
			newMu.Unlock()
		}()
	}
	wg.Wait()
	if len(newConns) == 0 {
		// 最快地址全部拨号失败（瞬时不可达），保持现有连接不动。
		return
	}

	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		for _, pc := range newConns {
			_ = pc.conn.Close()
		}
		return
	}
	// 先建后删：新连接入池后再关闭旧连接，切换期间池始终可借出。
	old := client.pool
	client.pool = newConns
	client.mu.Unlock()
	for _, pc := range old {
		_ = pc.conn.Close()
	}
}

// refillPool 补建一条缺失连接：先快速判断是否需要补建，需要时在锁外拨号与
// 握手（网络操作不阻塞借还），成功后加锁入池；池已关闭或已满时关闭多余连接。
func (client *Client) refillPool() {
	client.mu.Lock()
	if client.closed || len(client.pool) >= client.opt.PoolSize {
		client.mu.Unlock()
		return
	}
	// 池内还有可用连接时复用其地址补建，避免每轮重新测速。
	known := ""
	if len(client.pool) > 0 {
		known = client.pool[0].addr
	}
	client.mu.Unlock()

	// 测速与拨号均为网络操作，在锁外执行，避免阻塞其他调用方借还连接。
	var address string
	if known != "" {
		address = known
	} else {
		// 池完全为空（初始建池失败或全部断连）：探活排序后取首个候选。
		probeTimeout := min(client.timeout(), 5*time.Second)
		ordered := probeOrder(client.addresses(), probeTimeout)
		if len(ordered) == 0 {
			return
		}
		address = ordered[0]
	}
	pc, _, err := client.buildPoolConn(address)
	if err != nil {
		// 单次补建失败不重试，等待下一轮；失败地址可能为瞬时不可达。
		return
	}
	client.mu.Lock()
	if !client.closed && len(client.pool) < client.opt.PoolSize {
		client.pool = append(client.pool, pc)
	} else {
		_ = pc.conn.Close()
	}
	client.mu.Unlock()
}

// exchange 在指定连接上完成一次请求-响应收发：编码请求、带超时写出发送、按帧读取响应头与负载，
// 压缩帧按需以 `zlib` 解压。
//
// 参数 `conn` 为执行收发的已连接 TCP 连接；参数 `builder` 负责协议请求的字节编码；返回的
// `respHeader` 为解码后的响应头，`payload` 为解压后的明文负载。请求编码失败、写重试超限、帧头或
// 负载读取失败、负载长度超过 `MessageMaxBytes`、解压失败等任一环节出错都会返回错误；连接字节流
// 已无法保证帧边界，调用方（`execute`）须丢弃该连接，避免残留字节破坏后续帧。
func (client *Client) exchange(conn net.Conn, builder proto.RequestBuilder) (respHeader *proto.RespHeader, payload []byte, err error) {
	sendData, err := builder.BuildRequest()
	if err != nil {
		return nil, nil, err
	}

	// 为本次收发设置整体截止时间，结束后清空超时以免影响后续操作。
	_ = conn.SetDeadline(time.Now().Add(client.timeout()))
	defer func() {
		_ = conn.SetDeadline(time.Time{})
	}()

	retryTimes := 0

	for {
		n, err := conn.Write(sendData)
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
	_, err = io.ReadFull(conn, headerBytes)
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
	_, err = io.ReadFull(conn, msgData)
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
	if client.closed {
		return nil, errors.New("client closed")
	}
	// 幂等：已有池直接复用，避免重复拨号。
	if len(client.pool) > 0 {
		return nil, nil
	}
	reply, err := client.buildPoolLocked()
	if err != nil {
		return nil, err
	}
	r, _ := reply.(*proto.Hello1Reply)
	return r, nil
}

// buildPoolLocked 在调用方已持池锁的前提下并发预建 `PoolSize` 条连接并启动补建循环。
//
// 候选地址先按可达性与延迟排序（探活超时限制 5 秒），再并发拨号前 `PoolSize` 个候选：
// 各地址拨号或握手失败即从候选剔除继续尝试下一地址；全部失败返回最后一次错误；部分成功则
// 以已建条数先行运行，由补建循环补齐。返回首个成功连接的握手应答；候选为空时返回 `nil` 与错误。
func (client *Client) buildPoolLocked() (any, error) {
	addresses := client.addresses()
	if len(addresses) == 0 {
		return nil, errors.New("no tcp address configured")
	}
	// 建池始终优先连接可达且低延迟的服务器：探活超时限制在 5 秒内，避免在
	// 大面积不可达的候选地址上浪费拨号时间（正式连接仍使用完整超时）。
	probeTimeout := min(client.timeout(), 5*time.Second)
	addresses = probeOrder(addresses, probeTimeout)
	poolSize := client.opt.PoolSize

	// 并发拨号 `PoolSize` 条连接，候选地址按排序轮转分配（第 `i` 条用 `addresses[i%len]`）：
	// 单地址超时不阻塞其余地址，整体耗时由最慢一次拨号决定；地址不足时同一地址并发建多条。
	limit := poolSize
	type poolResult struct {
		pc    *pooledConn
		reply any
		err   error
	}
	results := make(chan poolResult, limit)
	var wg sync.WaitGroup
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			pc, reply, err := client.buildPoolConn(addr)
			results <- poolResult{pc: pc, reply: reply, err: err}
		}(addresses[i%len(addresses)])
	}
	wg.Wait()
	close(results)

	var lastErr error
	var firstReply any
	for r := range results {
		if r.err != nil {
			// 该地址拨号或握手失败，由补建循环继续尝试其余候选。
			lastErr = r.err
			continue
		}
		if firstReply == nil {
			firstReply = r.reply
		}
		client.pool = append(client.pool, r.pc)
	}
	if len(client.pool) == 0 {
		if lastErr == nil {
			lastErr = errors.New("no available tcp address")
		}
		return nil, lastErr
	}
	// 启动补建循环（仅一次）。调用方已持池锁，直接启动避免二次加锁；
	// 停止信号通道先建好，保证循环可按 `Disconnect` 退出。
	if client.done == nil {
		client.done = make(chan struct{})
	}
	client.refill.Do(func() { go client.refillLoop() })
	return firstReply, nil
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
	if client.closed {
		return nil, errors.New("client closed")
	}
	// 幂等：已有池直接复用，避免重复拨号。
	if len(client.pool) > 0 {
		return nil, nil
	}
	reply, err := client.buildPoolLocked()
	if err != nil {
		return nil, err
	}
	r, _ := reply.(*proto.ExLoginReply)
	return r, nil
}

// Disconnect 关闭连接池与补建循环，并级联关闭关联的内嵌主站、扩展站连接。
//
// 本方法无业务入参；返回关闭过程中遇到的首个错误，后续内嵌连接的关闭错误不会覆盖先出现的错误；
// 已关闭的内嵌连接会跳过重复关闭。关闭后客户端不可再借出连接，需重新 `Connect` 建池。
func (client *Client) Disconnect() error {
	client.mu.Lock()
	client.closed = true
	if client.done != nil {
		close(client.done)
		client.done = nil
	}
	pool := client.pool
	client.pool = nil
	main := client.main
	ex := client.ex
	client.main = nil
	client.ex = nil
	client.mu.Unlock()

	var err error
	for _, pc := range pool {
		if closeErr := pc.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
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
