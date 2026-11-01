package gotdx

import (
	"errors"
	"reflect"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// ErrMarketCodeCount 表示批量请求时传入的市场（或分类）列表与证券代码列表长度不一致，由
// `makeStocks`、`makeExStocks` 在配对前校验长度时返回。
var ErrMarketCodeCount = errors.New("market code count error")

// execute 完成一次协议请求-响应流程：从池内借出一条连接（借出即移出池）、持该连接
// 串行锁执行收发，解析成功后归还池内；收发或解析失败时由池销毁该条并后台补建。
//
// 解析成功后记录本次借出连接地址为"最近成功响应的服务器"（`lastDataAddr`），后续借出
// 定向优先同地址连接，让请求稳定落回已验证节点；该地址仅在重平衡切换或连接丢弃后
// 自然失效。
//
// 空数据换节点：应答为空数据（通用判空规则见 `isEmptyReply`）时解除定向并换池内另一条
// 连接重试（最多 2 次），命中非空后重新定向；重试仍空（该日期确实无成交）返回首次应答。
// 部分服务器对较早历史日期数据不全，多地址池下换节点可命中数据完整的服务器（老库固定
// 单节点必然命中其节点，新库以换节点重试达成同等稳定）。
//
// 参数 `protocol` 为待执行的协议对象，泛型 `T` 为应答类型；池内暂无连接（未连接或全部被
// 借出/丢弃待补建）或收发、解析失败时返回 `T` 零值与错误。
func (client *Client) execute[T any](protocol proto.Protocol[T]) (T, error) {
	var zero T
	reply, addr, err := client.executeOnce(protocol)
	if err != nil {
		return zero, err
	}
	if isEmptyReply(reply) {
		// 空数据换节点重试：解除对当前节点的定向，换池内另一条连接再查。
		// 命中非空后重新定向到该节点；重试仍空返回首次应答（该日期确实无成交）。
		client.clearDataAddr(addr)
		for i := 0; i < 2; i++ {
			if r2, addr2, err2 := client.executeOnce(protocol); err2 == nil {
				if !isEmptyReply(r2) {
					client.setDataAddr(addr2)
					return r2, nil
				}
			}
		}
		return reply, nil
	}
	// 本次连接即为可用节点，记录供定向优先复用，让后续请求稳定落回该服务器。
	client.setDataAddr(addr)
	return reply, nil
}

// isEmptyReply 通过反射报告协议应答是否为空数据：历史、分时、逐笔类协议的应答
// 结构统一携带 `Count uint16` 字段，`Count` 为零即无任何记录；MAC 偏移 K 线应答
// 无 `Count` 字段，按 `Returned` 字段判空。反射仅读取一个 `uint16` 字段，单次
// 开销可忽略。应答类型不含上述字段时按非空处理（不触发换节点）。
func isEmptyReply(v any) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	if f := rv.FieldByName("Count"); f.IsValid() && f.Kind() == reflect.Uint16 {
		return f.Uint() == 0
	}
	if f := rv.FieldByName("Returned"); f.IsValid() && f.Kind() == reflect.Uint32 {
		return f.Uint() == 0
	}
	return false
}

// executeOnce 执行单次协议请求-响应：借出连接、收发、解析并归还/销毁连接，由
// `execute` 编排调用。返回值依次为应答、本次借出连接的实际地址与错误（无错误时
// 为 `nil`）。
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

// setDataAddr 记录指定地址为最近成功响应的服务器，供借出定向优先复用。
func (client *Client) setDataAddr(addr string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if addr != "" {
		client.lastDataAddr = addr
	}
}

// clearDataAddr 清除对指定地址的定向记录：该地址返回空数据（或已失效）时调用，
// 使后续借出回退轮询，避免请求继续固定在数据不全的节点上。
func (client *Client) clearDataAddr(addr string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.lastDataAddr == addr {
		client.lastDataAddr = ""
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
