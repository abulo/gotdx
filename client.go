package gotdx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
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
	lastReachable       []string // 最近一次探活确认可达的地址列表，供重平衡与补建收敛候选
	rebalanceTick       int      // 重平衡执行计数，每 5 次触发一次全量探活刷新可达集
}

// pooledConn 为池内一条 TCP 连接：自带串行锁，保证该连接上的请求严格
// 先写后读、帧边界不被并发破坏。
type pooledConn struct {
	conn  net.Conn
	mu    sync.Mutex // 该连接串行锁
	addr  string     // 实际连通的 `host:port` 地址
	hello any        // 建连时模式握手的应答，仅建池返回首个应答时使用
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
// buildPoolConn 按默认超时建连：拨号后开启 TCP 保活并执行当前模式握手，返回
// 已握手的连接与握手应答。建池慢路径、补建与切换使用；快路径走
// `buildPoolConnWithTimeout`（短拨号超时排除偶发慢样本）。
func (client *Client) buildPoolConn(address string) (*pooledConn, any, error) {
	return client.buildPoolConnWithTimeout(address, client.timeout())
}

// buildPoolConnWithTimeout 在指定拨号超时内建连：拨号后开启 TCP 保活并执行当前
// 模式握手，返回已握手的连接与握手应答。
func (client *Client) buildPoolConnWithTimeout(address string, dialTimeout time.Duration) (*pooledConn, any, error) {
	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return nil, nil, err
	}
	// 开启 TCP 保活：行情长连接可能长时间无请求，避免中间设备（NAT/防火墙）
	// 或服务端静默回收空闲连接导致下次请求失败。
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(15 * time.Second)
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
	heartbeatTicker := time.NewTicker(30 * time.Second)
	rebalanceTicker := time.NewTicker(time.Minute)
	defer refillTicker.Stop()
	defer heartbeatTicker.Stop()
	defer rebalanceTicker.Stop()
	// 启动即并发补满池余量（建池同步只建 1 条），让高并发场景无需等待每秒补建；
	// 单条失败不阻塞，余量由后续补建循环继续补齐。
	client.refillPoolBulk()
	client.refillPool()
	for {
		select {
		case <-client.done:
			return
		case <-refillTicker.C:
			client.refillPool()
		case <-heartbeatTicker.C:
			client.heartbeatPool()
		case <-rebalanceTicker.C:
			client.rebalancePool()
		}
	}
}

// heartbeatTimeout 为单次应用层心跳的读写超时：僵死连接检测要快，避免占用
// 连接与补建窗口太久，正常连接一次心跳仅一个往返耗时。
const heartbeatTimeout = 2 * time.Second

// fastDialTimeout 为建池快路径单次拨号超时：行情服务器正常拨号在几十毫秒量级，
// 偶发秒级慢响应是服务器间歇波动，短超时直接排除该样本，避免建池踩中慢服务器；
// 全部候选短超时失败转慢路径流水线探活兜底。
const fastDialTimeout = 500 * time.Millisecond

// heartbeatPool 对池内空闲连接执行应用层心跳：主站/扩展站发送握手协议请求并以
// 短超时读回完整响应帧，能完整收发即视为连接存活；失败即丢弃该连接，由补建循环
// 补齐。MAC 模式无握手协议跳过（由 TCP keepalive 兜底）。连接正被业务请求占用时
// 跳过（`TryLock`），避免抢占在途请求。
func (client *Client) heartbeatPool() {
	client.mu.Lock()
	conns := append([]*pooledConn(nil), client.pool...)
	client.mu.Unlock()
	for _, pc := range conns {
		// 心跳只在连接空闲时进行：`TryLock` 失败表示该连接正在执行业务请求。
		if !pc.mu.TryLock() {
			continue
		}
		err := client.heartbeatConn(pc)
		pc.mu.Unlock()
		if err != nil {
			client.discard(pc)
		}
	}
}

// heartbeatConn 在单条连接上执行应用层心跳：发送当前模式握手请求并以 `heartbeatTimeout`
// 超时读回完整响应帧，能完整收发即返回 `nil`，不校验响应内容（验证的是连接存活而非
// 应答正确性）。MAC 模式无握手协议，直接返回 `nil`（由 TCP keepalive 兜底）。
func (client *Client) heartbeatConn(pc *pooledConn) error {
	var obj proto.RequestBuilder
	switch client.mode {
	case clientModeEx:
		obj = proto.NewExLogin()
	case clientModeMacMain, clientModeMacEx:
		return nil
	default:
		obj = proto.NewHello1()
	}
	sendData, err := obj.BuildRequest()
	if err != nil {
		return err
	}
	// 心跳使用独立短超时：僵死连接在 2 秒内即可判定，不必等到业务超时。
	_ = pc.conn.SetDeadline(time.Now().Add(heartbeatTimeout))
	defer func() { _ = pc.conn.SetDeadline(time.Time{}) }()

	if _, err := pc.conn.Write(sendData); err != nil {
		return err
	}
	headerBytes := make([]byte, proto.MessageHeaderBytes)
	if _, err := io.ReadFull(pc.conn, headerBytes); err != nil {
		return err
	}
	headerBuf := bytes.NewReader(headerBytes)
	var header proto.RespHeader
	if err := binary.Read(headerBuf, binary.LittleEndian, &header); err != nil {
		return err
	}
	// 负载长度超出协议上限视为非法数据，防止畸形长度触发过大的内存分配。
	if header.ZipSize > proto.MessageMaxBytes {
		return types.ErrBadData
	}
	msgData := make([]byte, header.ZipSize)
	if _, err := io.ReadFull(pc.conn, msgData); err != nil {
		return err
	}
	// 压缩帧解压后丢弃内容：心跳只验证连接能完整收发，不关心应答语义。
	if header.ZipSize != header.UnZipSize {
		r, err := zlib.NewReader(bytes.NewReader(msgData))
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(io.Discard, r)
		_ = r.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// refillPoolBulk 启动时并发补齐池余量：与池内首条连接同地址并发拨号
// `PoolSize-当前条数` 条，只入成功者；全部失败不重试，由补建循环后续补齐。
func (client *Client) refillPoolBulk() {
	client.mu.Lock()
	if client.closed || len(client.pool) >= client.opt.PoolSize {
		client.mu.Unlock()
		return
	}
	known := ""
	if len(client.pool) > 0 {
		known = client.pool[0].addr
	}
	need := client.opt.PoolSize - len(client.pool)
	client.mu.Unlock()
	if known == "" || need <= 0 {
		return
	}

	// 网络操作为主，在锁外并发拨号；只收集成功连接。
	resultsCh := make(chan *pooledConn, need)
	var wg sync.WaitGroup
	for i := 0; i < need; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pc, _, err := client.buildPoolConn(known)
			if err != nil {
				return
			}
			resultsCh <- pc
		}()
	}
	wg.Wait()
	close(resultsCh)

	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		for pc := range resultsCh {
			_ = pc.conn.Close()
		}
		return
	}
	for pc := range resultsCh {
		if len(client.pool) < client.opt.PoolSize {
			client.pool = append(client.pool, pc)
		} else {
			// 池已满（补建循环同时入池）时关闭多余连接。
			_ = pc.conn.Close()
		}
	}
	client.mu.Unlock()
}

// rebalancePool 每分钟对候选地址重新测速一次，把连接池切换到当前最快的服务器。
//
// 候选收敛为"池内地址 + 最近一次可达地址"，每 5 次触发一次全量探活刷新可达集；
// 并发探活并按可达性、延迟排序（探活超时限制 5 秒），取首个可达地址为最快目标。
// 池内连接已全部指向该地址时不作调整。否则并发预建 `PoolSize` 条指向最快地址的
// 连接，全部失败保持现状；至少建成一条则先入池再关闭旧连接（先建后删，切换期间
// 不出现空窗），未建满的余量由补建循环继续补齐。
func (client *Client) rebalancePool() {
	// 探活与拨号均为网络操作，在锁外执行，避免阻塞其他调用方借还连接。
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return
	}
	client.rebalanceTick++
	fullProbe := client.rebalanceTick%5 == 0
	poolAddrs := make([]string, 0, len(client.pool))
	for _, pc := range client.pool {
		duplicated := false
		for _, a := range poolAddrs {
			if a == pc.addr {
				duplicated = true
				break
			}
		}
		if !duplicated {
			poolAddrs = append(poolAddrs, pc.addr)
		}
	}
	prevReachable := append([]string(nil), client.lastReachable...)
	client.mu.Unlock()

	// 构建候选集：默认收敛为池内地址与历史可达地址；每 5 次执行一次全量
	// 探活（覆盖配置地址池），以便发现网络变化后新增的可达服务器。
	var candidates []string
	if !fullProbe {
		seen := make(map[string]struct{}, len(poolAddrs)+len(prevReachable))
		candidates = make([]string, 0, len(poolAddrs)+len(prevReachable))
		for _, a := range poolAddrs {
			if _, ok := seen[a]; ok {
				continue
			}
			candidates = append(candidates, a)
			seen[a] = struct{}{}
		}
		for _, a := range prevReachable {
			if _, ok := seen[a]; ok {
				continue
			}
			candidates = append(candidates, a)
			seen[a] = struct{}{}
		}
	} else {
		candidates = client.addresses()
	}
	if len(candidates) == 0 {
		return
	}

	probeTimeout := min(client.timeout(), fastDialTimeout)
	verified := client.probeReachable(candidates, probeTimeout)
	var fastest string
	reachable := make([]string, 0, len(verified))
	for _, r := range verified {
		if !r.reachable {
			continue
		}
		if fastest == "" {
			fastest = r.address
		}
		reachable = append(reachable, r.address)
	}
	client.mu.Lock()
	if len(reachable) > 0 {
		client.lastReachable = reachable
	}
	client.mu.Unlock()
	if fastest == "" {
		// 收敛候选全部不可达：保持现有连接不动。
		return
	}

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
	// 锁内拷贝最近一次可达记录，供池全空时收敛候选使用。
	lastReachable := append([]string(nil), client.lastReachable...)
	client.mu.Unlock()

	// 测速与拨号均为网络操作，在锁外执行，避免阻塞其他调用方借还连接。
	var address string
	if known != "" {
		address = known
	} else {
		// 池完全为空（初始建池失败或全部断连）：优先用最近一次可达地址收敛候选，
		// 避免每轮补建都全量探活；无可达记录时回退配置地址池。流水线探测首个可用
		// 连接并直接复用（零额外建连），探活超时 2 秒让全灭场景快速判负。
		probeTimeout := min(client.timeout(), fastDialTimeout)
		candidates := client.addresses()
		if len(lastReachable) > 0 {
			candidates = lastReachable
		}
		pc, _ := client.probeFirstReachable(candidates, probeTimeout)
		if pc == nil {
			return
		}
		client.mu.Lock()
		if !client.closed && len(client.pool) < client.opt.PoolSize {
			client.pool = append(client.pool, pc)
		} else {
			_ = pc.conn.Close()
		}
		client.mu.Unlock()
		return
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

// buildPoolLocked 在调用方已持池锁的前提下按候选顺序逐级建池并启动补建循环。
//
// 探活升级为"拨号 + 握手验证"：先并发拨号筛选可达地址（探活超时限制 5 秒），再对可达
// 地址并发执行模式握手，握手成功才判定为可用并按总耗时（拨号 + 握手）排序。建池从最快
// 可用地址开始，每个候选并发拨号 `PoolSize` 条，建成至少一条即停止（先建成即降级可用，
// 未建满的余量由补建循环补齐）；当前候选全部失败时按排序逐级回退下一候选。全部候选均
// 失败返回最后一次错误与 `nil`。返回首个成功连接的握手应答；候选为空时返回 `nil` 与错误。
func (client *Client) buildPoolLocked() (any, error) {
	addresses := client.addresses()
	if len(addresses) == 0 {
		return nil, errors.New("no tcp address configured")
	}

	// 快路径：并行拨号配置前 3 个候选，收到第一个成功即返回——波动网络下无需等待
	// 首地址超时即可命中可达服务器（原库为串行逐候选拨号）；稳定网络下首地址先
	// 成功，耗时与单拨相同。只同步建 1 条返回，余量由补建循环异步补齐；其余候选
	// 的并发结果由清理 goroutine 收集：成功连接入池复用（不超 `PoolSize`），失败关闭。
	type connRes struct {
		pc    *pooledConn
		reply any
		err   error
	}
	limit := 3
	if len(addresses) < limit {
		limit = len(addresses)
	}
	resultsCh := make(chan connRes, limit)
	for i := 0; i < limit; i++ {
		addr := addresses[i]
		go func() {
			// 快路径拨号用短超时：行情服务器正常拨号在几十毫秒量级，偶发 1 秒级
			// 慢响应是服务器间歇波动，直接排除而非踩中样本（原库固定连稳定服务器，
			// 分布窄正源于此）；全部短超时失败转慢路径探活兜底。
			pc, reply, err := client.buildPoolConnWithTimeout(addr, fastDialTimeout)
			resultsCh <- connRes{pc: pc, reply: reply, err: err}
		}()
	}
	consumed := 0
	for {
		r := <-resultsCh
		consumed++
		if r.err != nil {
			if consumed == limit {
				break // 并行快建全部失败，转慢路径探活回退
			}
			continue
		}
		// 收到第一个成功即返回；剩余并发结果由清理 goroutine 收尾。
		if remaining := limit - consumed; remaining > 0 {
			go func(remaining int) {
				for j := 0; j < remaining; j++ {
					rr := <-resultsCh
					if rr.err != nil {
						continue
					}
					client.mu.Lock()
					if !client.closed && len(client.pool) < client.opt.PoolSize {
						client.pool = append(client.pool, rr.pc)
					} else {
						_ = rr.pc.conn.Close()
					}
					client.mu.Unlock()
				}
			}(remaining)
		}
		client.pool = append(client.pool, r.pc)
		client.startRefillLoop()
		return r.reply, nil
	}

	// 慢路径：并行快建全部失败时流水线探测首个可用连接——拨号成功即握手、首个
	// 握手成功者直接复用（零额外建连），探活超时收紧为 2 秒让全灭场景快速判负。
	probeTimeout := min(client.timeout(), fastDialTimeout)
	first, reply := client.probeFirstReachable(addresses, probeTimeout)
	if first == nil {
		return nil, fmt.Errorf("no connection to any candidate")
	}
	client.lastReachable = []string{first.addr}
	client.pool = append(client.pool, first)
	client.startRefillLoop()
	return reply, nil
}

// startRefillLoop 启动补建循环（仅一次）。调用方已持池锁，直接启动避免二次加锁；
// 停止信号通道先建好，保证循环可按 `Disconnect` 退出。
func (client *Client) startRefillLoop() {
	if client.done == nil {
		client.done = make(chan struct{})
	}
	client.refill.Do(func() { go client.refillLoop() })
}

// probeResult 为一次探活验证的结果：`address` 为 `host:port` 地址，`reachable` 表示
// 拨号与模式握手均成功，`latency` 为总耗时（握手成功时为拨号加握手耗时，否则为拨号耗时）。
type probeResult struct {
	address   string
	reachable bool
	latency   time.Duration
}

// probeFirstReachable 流水线探测首个可用连接：并发拨号候选，任一地址拨号成功立即
// 执行当前模式握手，首个握手成功者连同握手应答一起返回——建池兜底与池空补建只求
// 最快可用，不等全部候选完成（与全量排序的 `probeReachable` 互补）。全部失败返回
// `nil`；提前返回后其余探测 goroutine 自然结束，多余连接自行关闭，不泄漏。
func (client *Client) probeFirstReachable(addresses []string, timeout time.Duration) (*pooledConn, any) {
	if len(addresses) == 0 {
		return nil, nil
	}
	type connRes struct {
		pc    *pooledConn
		reply any
	}
	resultsCh := make(chan connRes, len(addresses))
	var wg sync.WaitGroup
	for _, address := range addresses {
		address := address
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", address, timeout)
			if err != nil {
				return
			}
			pc := &pooledConn{conn: conn, addr: address}
			pc.mu.Lock()
			reply, hErr := client.handshakeOn(pc)
			pc.mu.Unlock()
			if hErr != nil {
				_ = conn.Close()
				return
			}
			select {
			case resultsCh <- connRes{pc: pc, reply: reply}:
			default:
				// 已有更快的成功者抢先返回，本条连接不再需要。
				_ = conn.Close()
			}
		}()
	}
	doneCh := make(chan struct{})
	go func() { wg.Wait(); close(doneCh) }()
	for {
		select {
		case r := <-resultsCh:
			return r.pc, r.reply
		case <-doneCh:
			return nil, nil
		}
	}
}

// probeReachable 探活验证候选地址：先并发拨号筛选（单次拨号超时 `timeout`），再对
// 拨号可达的地址并发执行当前模式握手。返回结果覆盖所有拨号成功的地址：握手成功者
// 在前（按拨号加握手总耗时升序），仅拨号成功但握手失败者排后（按拨号耗时升序），
// 供建池逐级回退时优先尝试已验证可用的地址。握手协议不存在的模式（MAC）以拨号
// 成功即判定可用。
func (client *Client) probeReachable(addresses []string, timeout time.Duration) []probeResult {
	if len(addresses) == 0 {
		return nil
	}
	// 阶段一：并发拨号筛选，保留 TCP 可达的地址并记录拨号耗时。
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		dialed = make([]probeResult, 0, len(addresses))
	)
	for _, address := range addresses {
		address := address
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			conn, err := net.DialTimeout("tcp", address, timeout)
			if err != nil {
				return
			}
			_ = conn.Close()
			mu.Lock()
			dialed = append(dialed, probeResult{address: address, latency: time.Since(started)})
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(dialed) == 0 {
		return nil
	}

	// 阶段二：对拨号可达的地址并发握手验证；握手成功标记可用并计入握手耗时。
	var verified []probeResult
	for _, r := range dialed {
		r := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", r.address, timeout)
			if err != nil {
				return
			}
			started := time.Now()
			pc := &pooledConn{conn: conn, addr: r.address}
			pc.mu.Lock()
			_, hErr := client.handshakeOn(pc)
			pc.mu.Unlock()
			_ = conn.Close()
			if hErr != nil {
				return
			}
			mu.Lock()
			verified = append(verified, probeResult{address: r.address, reachable: true, latency: time.Since(started)})
			mu.Unlock()
		}()
	}
	wg.Wait()

	// 合并：握手成功者在前（总耗时升序），仅拨号成功者排后（拨号耗时升序）。
	merged := make([]probeResult, 0, len(dialed))
	merged = append(merged, verified...)
	for _, r := range dialed {
		duplicated := false
		for _, v := range merged {
			if v.address == r.address {
				duplicated = true
				break
			}
		}
		if !duplicated {
			merged = append(merged, r)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].reachable != merged[j].reachable {
			return merged[i].reachable
		}
		return merged[i].latency < merged[j].latency
	})
	return merged
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
