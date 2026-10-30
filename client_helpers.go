package gotdx

import (
	"errors"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// ErrMarketCodeCount 表示传入的市场列表与证券代码列表长度不一致。
var ErrMarketCodeCount = errors.New("market code count error")

// execute 加锁执行一次协议请求，保证同一连接上的请求串行化，避免并发读写打乱协议帧。
func (client *Client) execute[T any](protocol proto.Protocol[T]) (T, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	return client.executeLocked(protocol)
}

// executeLocked 在调用方已持锁的前提下执行协议流程：收发数据、解析响应、返回业务应答对象。
func (client *Client) executeLocked[T any](protocol proto.Protocol[T]) (T, error) {
	var zero T

	header, payload, err := client.exchange(protocol)
	if err != nil {
		return zero, err
	}
	// 协议解析失败时返回零值与错误，由上层决定是否重建连接。
	if err := protocol.ParseResponse(header, payload); err != nil {
		return zero, err
	}
	return protocol.Response(), nil
}

func makeCode6(code string) [6]byte {
	var out [6]byte
	copy(out[:], code)
	return out
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

// makeStocks 把市场与代码两两配对组装成主站证券列表；两者长度不一致时返回 ErrMarketCodeCount。
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

// makeExStocks 把扩展分类与代码两两配对组装成扩展站证券列表；两者长度不一致时返回错误。
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

// quotesSortReverse 把布尔排序方向转换为协议方向码：代码排序不区分升降序，恒为 0；
// 其余排序 1 为升序、2 为降序。
func quotesSortReverse(sortType uint16, reverse bool) uint16 {
	if sortType == types.SortCode {
		return 0
	}
	if reverse {
		return 2
	}
	return 1
}
