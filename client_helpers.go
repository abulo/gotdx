package gotdx

import (
	"errors"
	"math/rand"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// ErrMarketCodeCount 表示批量请求时传入的市场（或分类）列表与证券代码列表长度不一致，由
// `makeStocks`、`makeExStocks` 在配对前校验长度时返回。
var ErrMarketCodeCount = errors.New("market code count error")

// execute 完成一次协议请求-响应流程：从池内借出一条连接（借出即移出池）、持该连接
// 串行锁执行收发，解析成功后归还池内；收发或解析失败时由池销毁该条并后台补建。
//
// 协议对象若实现 `proto.EmptyResponseAware` 且最近一次应答为空数据（部分扩展站对较早
// 日期数据不全），执行层自动换节点重试：先换池内另一条连接（定向优先下大概率借出
// 不同服务器），仍空则从配置地址池换地址临时建连再查（优先最近一次命中数据的服务器）。
// 重试仍空（该日期确实无成交）时返回首次应答，保持"一次调用一次结果"的语义。
//
// 参数 `protocol` 为待执行的协议对象，泛型 `T` 为应答类型；池内暂无连接（未连接或全部被
// 借出/丢弃待补建）或收发、解析失败时返回 `T` 零值与错误。
func (client *Client) execute[T any](protocol proto.Protocol[T]) (T, error) {
	var zero T
	reply, addr, err := client.executeOnce(protocol)
	if err != nil {
		return zero, err
	}
	// 空响应换节点重试：仅对声明了 `EmptyResponseAware` 的协议生效，其余协议单次执行。
	if retrier, ok := any(protocol).(proto.EmptyResponseAware); ok && retrier.IsEmptyResponse() {
		// 池内重试：换池内另一条连接（多地址池下大概率指向不同服务器）。
		// 复用同一协议对象串行执行，多次解析覆盖应答，最终返回最后一次结果。
		for i := 0; i < 2; i++ {
			if r2, addr2, err2 := client.executeOnce(protocol); err2 == nil && !retrier.IsEmptyResponse() {
				client.setDataAddr(addr2)
				return r2, nil
			}
		}
		// 临时换地址重试：优先最近一次命中数据的服务器，其次从配置地址池随机换
		// 地址临时建连（最多两个随机候选）。临时连接用完即关，不污染池。
		candidates := make([]string, 0, 3)
		if addr := client.dataAddr(); addr != "" {
			candidates = append(candidates, addr)
		}
		for i := 0; i < 2; i++ {
			if addr := client.randomOtherAddress(client.CurrentAddress()); addr != "" {
				candidates = append(candidates, addr)
			}
		}
		for _, a := range candidates {
			if r3, err3 := client.executeTemporary(a, protocol); err3 == nil && !retrier.IsEmptyResponse() {
				client.setDataAddr(a)
				return r3, nil
			}
		}
		return reply, nil
	}
	// 非空响应（或未声明空感知的协议）：本次连接即为可用节点，记录供定向优先
	// 与空重试复用，让后续请求稳定落回该服务器。
	client.setDataAddr(addr)
	return reply, nil
}

// executeOnce 执行单次协议请求-响应：借出连接、收发、解析并归还/销毁连接，不包含
// 空响应重试逻辑，由 `execute` 编排调用。返回值依次为应答、本次借出连接的实际地址
// 与错误（无错误时为 `nil`）。
func (client *Client) executeOnce[T any](protocol proto.Protocol[T]) (T, string, error) {
	var zero T
	pc, err := client.acquire()
	if err != nil {
		return zero, "", err
	}
	pc.mu.Lock()
	header, payload, err := client.exchange(pc.conn, protocol)
	if err == nil {
		err = protocol.ParseResponse(header, payload)
	}
	if err != nil {
		// 收发或解析失败后该连接帧边界不可信，销毁并交后台补建。
		client.discard(pc)
		pc.mu.Unlock()
		return zero, "", err
	}
	pc.mu.Unlock()
	client.release(pc)
	return protocol.Response(), pc.addr, nil
}

func makeCode6(code string) [6]byte {
	var out [6]byte
	copy(out[:], code)
	return out
}

// executeTemporary 在指定地址上临时建立连接（含模式握手）执行一次协议请求后立即关闭，
// 用于空数据重试兜底：池内连接指向的服务器对部分历史日期数据不全时，临时换地址再查。
// 临时连接不进入池，执行结束即释放，避免影响池容量与心跳/重平衡巡检。
func (client *Client) executeTemporary[T any](addr string, protocol proto.Protocol[T]) (T, error) {
	var zero T
	pc, _, err := client.buildPoolConn(addr)
	if err != nil {
		return zero, err
	}
	defer func() { _ = pc.conn.Close() }()
	pc.mu.Lock()
	defer pc.mu.Unlock()
	header, payload, err := client.exchange(pc.conn, protocol)
	if err == nil {
		err = protocol.ParseResponse(header, payload)
	}
	if err != nil {
		return zero, err
	}
	return protocol.Response(), nil
}

// randomOtherAddress 从配置地址池中随机返回一个与 `current` 不同的地址；地址池仅含
// `current`（或无其他地址）时返回空字符串。随机化避免重试固定落在同一波动窗口。
func (client *Client) randomOtherAddress(current string) string {
	others := make([]string, 0, 4)
	for _, a := range client.addresses() {
		if a != current {
			others = append(others, a)
		}
	}
	if len(others) == 0 {
		return ""
	}
	return others[rand.Intn(len(others))]
}

// dataAddr 返回最近一次命中数据的服务器地址；无记录时返回空字符串。
func (client *Client) dataAddr() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.lastDataAddr
}

// setDataAddr 显式记录指定地址为最近命中数据的服务器（临时重试命中后回写）。
func (client *Client) setDataAddr(addr string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if addr != "" {
		client.lastDataAddr = addr
	}
}

func makeCode9(code string) [9]byte {
	var out [9]byte
	copy(out[:], code)
	return out
}

func makeCode23(code string) [23]byte {
	var out [23]byte
	copy(out[:], code)
	return out
}

func makeCode22(code string) [22]byte {
	var out [22]byte
	copy(out[:], code)
	return out
}

func makeFixed40(value string) [40]byte {
	var out [40]byte
	copy(out[:], value)
	return out
}

func makeFixed80(value string) [80]byte {
	var out [80]byte
	copy(out[:], value)
	return out
}

func makeFixed300(value string) [300]byte {
	var out [300]byte
	copy(out[:], value)
	return out
}

func makeFixed43(value string) [43]byte {
	var out [43]byte
	copy(out[:], value)
	return out
}

// makeStocks 把市场与代码按下标两两配对组装成主站证券列表。
//
// 参数 `markets` 为市场代码序列，取值见 `types.Market*`；参数 `codes` 为六位证券代码序列，与市场
// 序列按位置一一对应。两者长度不一致时返回 `nil` 与 `ErrMarketCodeCount`。
func makeStocks(markets []uint8, codes []string) ([]proto.Stock, error) {
	if len(markets) != len(codes) {
		return nil, ErrMarketCodeCount
	}

	stocks := make([]proto.Stock, 0, len(markets))
	for i, market := range markets {
		stocks = append(stocks, proto.Stock{
			Market: market,
			Code:   codes[i],
		})
	}
	return stocks, nil
}

// makeExStocks 把扩展分类与代码按下标两两配对组装成扩展站证券列表。
//
// 参数 `categories` 为扩展市场类别序列，取值见 `types.ExMarket*` 与 `types.ExCategory*`；参数
// `codes` 为扩展站证券代码序列，与分类序列按位置一一对应。两者长度不一致时返回 `nil` 与
// `ErrMarketCodeCount`。
func makeExStocks(categories []uint8, codes []string) ([]proto.ExStock, error) {
	if len(categories) != len(codes) {
		return nil, ErrMarketCodeCount
	}

	stocks := make([]proto.ExStock, 0, len(categories))
	for i, category := range categories {
		stocks = append(stocks, proto.ExStock{
			Category: category,
			Code:     codes[i],
		})
	}
	return stocks, nil
}

// quotesSortReverse 把布尔排序方向转换为协议方向码。
//
// 参数 `sortType` 为排序字段编号，取值见 `types.Sort*`；参数 `reverse` 为 `true` 表示降序。代码排序
// （`types.SortCode`）不区分升降序，恒返回 0；其余字段 `reverse` 为 `false` 返回 1（升序），为
// `true` 返回 2（降序）。
func quotesSortReverse(sortType uint16, reverse bool) uint16 {
	if sortType == types.SortCode {
		return 0
	}
	if reverse {
		return 2
	}
	return 1
}
