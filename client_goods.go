package gotdx

import (
	"fmt"

	"github.com/bensema/gotdx/proto"
)

const (
	// DefaultGoodsHistoryTransactionCount 是 GoodsHistoryTransaction 在 `count` 为 0 时使用的默认历史成交条数。
	DefaultGoodsHistoryTransactionCount uint32 = 2000
	// MaxGoodsVarietiesCount 是 GoodsVarieties 单次请求允许的最大条数，超出即返回参数错误。
	MaxGoodsVarietiesCount uint32 = 1000
)

// GoodsCount 查询扩展市场商品总数，内部委托 ExCount 实现。
func (client *Client) GoodsCount() (uint32, error) {
	return client.ExCount()
}

// GoodsCategoryList 查询扩展市场商品分类列表，内部委托 ExCategoryList 实现。
func (client *Client) GoodsCategoryList() ([]proto.ExCategoryItem, error) {
	return client.ExCategoryList()
}

// GoodsList 从 `start` 偏移开始拉取扩展市场商品列表，内部委托 ExList 实现。
func (client *Client) GoodsList(start uint32, count uint16) ([]proto.ExListItem, error) {
	return client.ExList(start, count)
}

// GoodsVarieties 查询扩展市场品种列表，内部委托 ExMapping2562 实现；
// `count` 超过 MaxGoodsVarietiesCount 时返回参数错误。
func (client *Client) GoodsVarieties(market uint16, start uint32, count uint32) ([]proto.ExMapping2562Item, error) {
	if count > MaxGoodsVarietiesCount {
		return nil, fmt.Errorf("goods varieties count exceeds %d: %d", MaxGoodsVarietiesCount, count)
	}
	return client.ExMapping2562(market, start, count)
}

// GoodsQuote 查询单个扩展市场商品报价，内部委托 ExQuote 实现。
func (client *Client) GoodsQuote(market uint8, code string) (*proto.ExQuoteItem, error) {
	return client.ExQuote(market, code)
}

// GoodsQuotes 批量查询扩展市场商品报价，内部委托 ExQuotes 实现；
// `markets` 与 `codes` 长度必须一致。
func (client *Client) GoodsQuotes(markets []uint8, codes []string) ([]proto.ExQuoteItem, error) {
	return client.ExQuotes(markets, codes)
}

// GoodsQuotesList 查询扩展市场商品排序行情列表，内部委托 ExQuotesList 实现；
// `start` 为起始偏移，`count` 为请求条数，`sortType` 为排序方式，`reverse` 为 `true` 时倒序排列。
func (client *Client) GoodsQuotesList(market uint8, start uint16, count uint16, sortType uint16, reverse bool) ([]proto.ExQuoteItem, error) {
	return client.ExQuotesList(market, start, count, sortType, reverse)
}

// GoodsKLine 查询扩展市场商品 MAC K 线，内部委托 MACSymbolBars 实现；
// `period` 为周期类型，`start` 为起始偏移，`count` 为请求条数，`times` 为周期倍数，`adjust` 为复权方式。
func (client *Client) GoodsKLine(market uint8, code string, period uint16, start uint32, count uint32, times uint16, adjust uint16) ([]proto.MACSymbolBar, error) {
	return client.MACSymbolBars(market, code, period, times, start, count, adjust)
}

// GoodsTickChart 查询扩展市场商品分时图，内部委托 MACQuotesWithDate 实现；
// `queryDate` 为 0 时返回实时分时，否则返回指定交易日的历史分时。
func (client *Client) GoodsTickChart(market uint8, code string, queryDate uint32) ([]proto.MACQuoteChartItem, error) {
	reply, err := client.MACQuotesWithDate(market, code, queryDate)
	if err != nil {
		return nil, err
	}
	return reply.ChartData, nil
}

// GoodsChartSampling 查询扩展市场商品分时缩略采样价格序列，内部委托 ExChartSampling 实现。
func (client *Client) GoodsChartSampling(market uint8, code string) ([]float64, error) {
	return client.ExChartSampling(market, code)
}

// GoodsHistoryTransaction 查询扩展市场商品历史成交，内部委托 MACTransactionsWithDate 实现；
// `count` 为 0 时取 DefaultGoodsHistoryTransactionCount 条，`queryDate` 指定交易日期。
func (client *Client) GoodsHistoryTransaction(market uint8, code string, queryDate uint32, count uint32) ([]proto.MACTransactionItem, error) {
	if count == 0 {
		count = DefaultGoodsHistoryTransactionCount
	}
	return client.MACTransactionsWithDate(market, code, 0, count, queryDate)
}
