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

// execute 完成一次协议请求-响应流程：从池内借出一条连接（借出即移出池）、持该连接串行锁
// 执行收发，解析成功后归还池内；收发或解析失败时由池销毁该条并后台补建。
//
// 参数 `protocol` 为待执行的协议对象，泛型 `T` 为应答类型；池内暂无连接（未连接或全部被
// 借出/丢弃待补建）或收发、解析失败时返回 `T` 零值与错误。
func (client *Client) execute[T any](protocol proto.Protocol[T]) (T, error) {
	var zero T
	pc, err := client.acquire()
	if err != nil {
		return zero, err
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
		return zero, err
	}
	pc.mu.Unlock()
	client.release(pc)
	return protocol.Response(), nil
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

// recordDataAddr 记录当前池地址为"最近命中数据的服务器"：历史查询成功返回非空数据后
// 调用，空数据重试时优先该地址，模拟老库固定好节点的稳定行为。
func (client *Client) recordDataAddr() {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.lastAddr != "" {
		client.lastDataAddr = client.lastAddr
	}
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
