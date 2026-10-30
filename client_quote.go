package gotdx

import (
	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// GetExchangeAnnouncement 拉取交易所公告内容；连接或协议执行失败时返回对应错误。
func (client *Client) GetExchangeAnnouncement() (*proto.ExchangeAnnouncementReply, error) {
	obj := proto.NewExchangeAnnouncement()
	return client.execute(obj)
}

// GetServerHeartbeat 发送心跳请求并返回服务端应答，用于维持连接活性；
// 连接或协议执行失败时返回对应错误。
func (client *Client) GetServerHeartbeat() (*proto.HeartBeatReply, error) {
	obj := proto.NewHeartBeat()
	return client.execute(obj)
}

// GetAnnouncement 拉取行情服务商公告；公告无内容时响应中的内容标记为不存在，
// 连接或协议执行失败时返回对应错误。
func (client *Client) GetAnnouncement() (*proto.AnnouncementReply, error) {
	obj := proto.NewAnnouncement()
	return client.execute(obj)
}

// GetServerInfo 拉取主站服务信息，包括延迟、服务端时间、地区与标识等字段；
// 连接或协议执行失败时返回对应错误。
func (client *Client) GetServerInfo() (*proto.InfoReply, error) {
	obj := proto.NewInfo()
	return client.execute(obj)
}

// GetTodoB 发送主站试验协议 `0x000b`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetTodoB() (*proto.RawDataReply, error) {
	obj := proto.NewTodoB()
	return client.execute(obj)
}

// GetTodoFDE 发送主站试验协议 `0x0fde`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetTodoFDE() (*proto.RawDataReply, error) {
	obj := proto.NewTodoFDE()
	return client.execute(obj)
}

// GetClient264B 发送客户端信息协议 `0x264b`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetClient264B() (*proto.RawDataReply, error) {
	obj := proto.NewClient264B()
	return client.execute(obj)
}

// GetClient26AC 发送客户端信息协议 `0x26ac`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetClient26AC() (*proto.RawDataReply, error) {
	obj := proto.NewClient26AC()
	return client.execute(obj)
}

// GetClient26AD 发送客户端信息协议 `0x26ad`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetClient26AD() (*proto.RawDataReply, error) {
	obj := proto.NewClient26AD()
	return client.execute(obj)
}

// GetClient26AE 发送客户端信息协议 `0x26ae`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetClient26AE() (*proto.RawDataReply, error) {
	obj := proto.NewClient26AE()
	return client.execute(obj)
}

// GetClient26B1 发送客户端信息协议 `0x26b1`，返回未做字段解析的原始响应；
// 该协议语义待确认，仅用于协议探测。
func (client *Client) GetClient26B1() (*proto.RawDataReply, error) {
	obj := proto.NewClient26B1()
	return client.execute(obj)
}

// GetSecurityCount 查询指定市场内的证券总数；`market` 取 `types` 包中的市场代码，
// 连接或协议执行失败时返回对应错误。
func (client *Client) GetSecurityCount(market uint8) (*proto.GetSecurityCountReply, error) {
	obj := proto.NewGetSecurityCount(&proto.GetSecurityCountRequest{Market: uint16(market)})
	return client.execute(obj)
}

// GetSecurityQuotes 批量查询证券盘口报价，内部委托 GetQuotesDetail 实现；
// `markets` 与 `codes` 长度必须一致，市场代码与证券代码配对发送，配对长度不一致时返回错误。
func (client *Client) GetSecurityQuotes(markets []uint8, codes []string) (*proto.GetSecurityQuotesReply, error) {
	return client.GetQuotesDetail(markets, codes)
}

// GetQuotesDetail 批量查询详细行情报价，返回每只证券的最新价、开高低、成交量、成交额
// 与换手率等盘口字段；`markets` 与 `codes` 长度必须一致，配对长度不一致时返回错误。
func (client *Client) GetQuotesDetail(markets []uint8, codes []string) (*proto.GetSecurityQuotesReply, error) {
	stocks, err := makeStocks(markets, codes)
	if err != nil {
		return nil, err
	}
	obj := proto.NewGetSecurityQuotes(&proto.GetSecurityQuotesRequest{StockList: stocks})
	return client.execute(obj)
}

// GetSecurityList 从 `start` 位置开始，按默认条数拉取指定市场内的证券代码与名称列表；
// `market` 取 `types` 包中的市场代码。
func (client *Client) GetSecurityList(market uint8, start uint16) (*proto.GetSecurityListReply, error) {
	return client.GetSecurityListRange(market, uint32(start), types.DefaultSecurityListCount)
}

// GetSecurityListOld 使用旧版协议从 `start` 位置开始拉取指定市场内的证券代码与名称列表。
func (client *Client) GetSecurityListOld(market uint8, start uint16) (*proto.GetSecurityListOldReply, error) {
	obj := proto.NewGetSecurityListOld(&proto.GetSecurityListOldRequest{Market: uint16(market), Start: start})
	return client.execute(obj)
}

// GetSecurityFeature452 按 `start` 偏移与 `count` 条数批量获取证券扩展信息；
// 返回条目 P1、P2 字段的具体业务含义语义待确认。
func (client *Client) GetSecurityFeature452(start uint32, count uint32) (*proto.GetSecurityFeature452Reply, error) {
	obj := proto.NewGetSecurityFeature452(&proto.GetSecurityFeature452Request{Start: start, Count: count})
	return client.execute(obj)
}

// GetSecurityListRange 从 `start` 位置开始拉取 `count` 条指定市场内的证券代码与名称列表；
// `market` 取 `types` 包中的市场代码。
func (client *Client) GetSecurityListRange(market uint8, start uint32, count uint32) (*proto.GetSecurityListReply, error) {
	obj := proto.NewGetSecurityList(&proto.GetSecurityListRequest{Market: uint16(market), Start: start, Count: count})
	return client.execute(obj)
}

// GetKLine 查询证券 K 线序列。
//
// `category` 取 `types` 包中的 K 线周期类型，`market` 取市场代码，`code` 为 6 位证券代码；
// `start` 为起始偏移，`count` 为请求条数，`times` 为周期倍数，`adjust` 取 `types` 包中的复权方式。
// 连接或协议执行失败时返回对应错误。
func (client *Client) GetKLine(category uint16, market uint8, code string, start uint16, count uint16, times uint16, adjust uint16) (*proto.GetSecurityBarsReply, error) {
	obj := proto.NewGetSecurityBars(&proto.GetSecurityBarsRequest{
		Market:   uint16(market),
		Code:     makeCode6(code),
		Category: category,
		Times:    times,
		Start:    start,
		Count:    count,
		Adjust:   adjust,
	})
	return client.execute(obj)
}

// GetSecurityBars 查询股票 K 线，固定周期倍数为 1、不复权；其余参数含义同 GetKLine。
func (client *Client) GetSecurityBars(category uint16, market uint8, code string, start uint16, count uint16) (*proto.GetSecurityBarsReply, error) {
	return client.GetKLine(category, market, code, start, count, 1, types.AdjustNone)
}

// GetSecurityBarsOffset 使用偏移量协议查询证券 K 线，参数含义同 GetKLine；
// 该协议响应结构与普通 K 线协议不同，由独立的 `Reply` 类型承载。
func (client *Client) GetSecurityBarsOffset(category uint16, market uint8, code string, start uint16, count uint16, times uint16, adjust uint16) (*proto.GetSecurityBarsOffsetReply, error) {
	obj := proto.NewGetSecurityBarsOffset(&proto.GetSecurityBarsOffsetRequest{
		Market:   uint16(market),
		Code:     makeCode6(code),
		Category: category,
		Times:    times,
		Start:    start,
		Count:    count,
		Adjust:   adjust,
	})
	return client.execute(obj)
}

// GetIndexBars 查询指数 K 线序列；`category` 取 `types` 包中的 K 线周期类型，
// `market` 取市场代码，`code` 为指数代码，`start` 为起始偏移，`count` 为请求条数，固定不复权。
func (client *Client) GetIndexBars(category uint16, market uint8, code string, start uint16, count uint16) (*proto.GetIndexBarsReply, error) {
	_, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	obj := proto.NewGetIndexBars(&proto.GetIndexBarsRequest{
		Market:   uint16(market),
		Code:     makeCode6(code),
		Category: category,
		Times:    1,
		Start:    start,
		Count:    count,
		Adjust:   types.AdjustNone,
	})
	return client.execute(obj)
}

// GetIndexMomentum 查询指定指数的动量数据；`market` 取市场代码，`code` 为指数代码。
func (client *Client) GetIndexMomentum(market uint8, code string) (*proto.GetIndexMomentumReply, error) {
	obj := proto.NewGetIndexMomentum(&proto.GetIndexMomentumRequest{Market: uint16(market), Code: makeCode6(code)})
	return client.execute(obj)
}

// GetIndexInfo 查询指定指数的概况信息；`market` 取市场代码，`code` 为指数代码。
func (client *Client) GetIndexInfo(market uint8, code string) (*proto.GetIndexInfoReply, error) {
	obj := proto.NewGetIndexInfo(&proto.GetIndexInfoRequest{Market: uint16(market), Code: makeCode6(code)})
	return client.execute(obj)
}

// GetVolumeProfile 查询指定证券的价位成交分布（量价分布）；`market` 取市场代码，`code` 为证券代码。
func (client *Client) GetVolumeProfile(market uint8, code string) (*proto.GetVolumeProfileReply, error) {
	obj := proto.NewGetVolumeProfile(&proto.GetVolumeProfileRequest{Market: uint16(market), Code: makeCode6(code)})
	return client.execute(obj)
}

// GetQuotesList 按分类拉取排序后的行情快照列表。
//
// `category` 为板块或市场分类，`start` 为起始偏移，`count` 为请求条数，`sortType` 为排序方式，
// `reverse` 为 `true` 时倒序排列，`filter` 为过滤条件；排序方向由 `sortType` 与 `reverse` 共同决定。
func (client *Client) GetQuotesList(category uint8, start uint16, count uint16, sortType uint16, reverse bool, filter uint16) (*proto.GetQuotesListReply, error) {
	obj := proto.NewGetQuotesList(&proto.GetQuotesListRequest{
		Category:    uint16(category),
		SortType:    sortType,
		Start:       start,
		Count:       count,
		SortReverse: quotesSortReverse(sortType, reverse),
		Filter:      filter,
	})
	return client.execute(obj)
}

// GetQuotes 批量查询证券行情快照；`markets` 与 `codes` 长度必须一致，
// 配对长度不一致时返回错误。
func (client *Client) GetQuotes(markets []uint8, codes []string) (*proto.GetQuotesReply, error) {
	stocks, err := makeStocks(markets, codes)
	if err != nil {
		return nil, err
	}
	obj := proto.NewGetQuotes(&proto.GetQuotesRequest{Stocks: stocks})
	return client.execute(obj)
}

// GetQuotesEncrypt 批量查询加密通道行情快照；`markets` 与 `codes` 长度必须一致，
// 配对长度不一致时返回错误。
func (client *Client) GetQuotesEncrypt(markets []uint8, codes []string) (*proto.GetQuotesEncryptReply, error) {
	stocks, err := makeStocks(markets, codes)
	if err != nil {
		return nil, err
	}
	obj := proto.NewGetQuotesEncrypt(&proto.GetQuotesEncryptRequest{Stocks: stocks})
	return client.execute(obj)
}

// GetMinuteTimeData 查询当日分时图数据，从第 0 点开始按默认条数拉取；
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetMinuteTimeData(market uint8, code string) (*proto.GetMinuteTimeDataReply, error) {
	return client.GetTickChart(market, code, 0, types.DefaultTickChartCount)
}

// GetTickChart 查询当日分时图数据；`start` 为分时点起始偏移，`count` 为请求点数，
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetTickChart(market uint8, code string, start uint16, count uint16) (*proto.GetMinuteTimeDataReply, error) {
	obj := proto.NewGetMinuteTimeData(&proto.GetMinuteTimeDataRequest{
		Market: uint16(market),
		Code:   makeCode6(code),
		Start:  start,
		Count:  count,
	})
	return client.execute(obj)
}

// GetHistoryMinuteTimeData 查询指定交易日的历史分时图数据；`date` 为交易日期，
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetHistoryMinuteTimeData(date uint32, market uint8, code string) (*proto.GetHistoryMinuteTimeDataReply, error) {
	return client.GetHistoryTickChart(date, market, code)
}

// GetHistoryTickChart 查询指定交易日的历史分时图数据；`date` 为交易日期，
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetHistoryTickChart(date uint32, market uint8, code string) (*proto.GetHistoryMinuteTimeDataReply, error) {
	obj := proto.NewGetHistoryMinuteTimeData(&proto.GetHistoryMinuteTimeDataRequest{
		Date:   int32(date),
		Market: market,
		Code:   makeCode6(code),
	})
	return client.execute(obj)
}

// GetChartSampling 查询指定证券的分时缩略采样价格序列；`market` 取市场代码，`code` 为证券代码。
func (client *Client) GetChartSampling(market uint8, code string) (*proto.GetChartSamplingReply, error) {
	obj := proto.NewGetChartSampling(&proto.GetChartSamplingRequest{Market: uint16(market), Code: makeCode6(code)})
	return client.execute(obj)
}

// GetAuction 查询指定证券的集合竞价记录；`start` 为起始偏移，`count` 为请求条数，
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetAuction(market uint8, code string, start uint32, count uint32) (*proto.GetAuctionReply, error) {
	obj := proto.NewGetAuction(&proto.GetAuctionRequest{
		Market: uint16(market),
		Code:   makeCode6(code),
		Start:  start,
		Count:  count,
	})
	return client.execute(obj)
}

// GetTopBoard 查询分类排行榜；`category` 为榜单分类，`size` 为每个榜单返回的条目数量。
func (client *Client) GetTopBoard(category uint8, size uint8) (*proto.GetTopBoardReply, error) {
	obj := proto.NewGetTopBoard(&proto.GetTopBoardRequest{Category: category, Size: size})
	return client.execute(obj)
}

// GetUnusual 查询异动监控（主力监控）记录；`start` 为起始偏移，`count` 为请求条数，
// `market` 取市场代码。
func (client *Client) GetUnusual(market uint8, start uint32, count uint32) (*proto.GetUnusualReply, error) {
	obj := proto.NewGetUnusual(&proto.GetUnusualRequest{Market: uint16(market), Start: start, Count: count})
	return client.execute(obj)
}

// GetTransactionData 查询当日分时逐笔成交；`start` 为起始偏移，`count` 为请求条数，
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetTransactionData(market uint8, code string, start uint16, count uint16) (*proto.GetTransactionDataReply, error) {
	obj := proto.NewGetTransactionData(&proto.GetTransactionDataRequest{
		Market: uint16(market),
		Code:   makeCode6(code),
		Start:  start,
		Count:  count,
	})
	return client.execute(obj)
}

// GetHistoryOrders 查询指定交易日的历史委托档位记录；`date` 为交易日期，
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetHistoryOrders(date uint32, market uint8, code string) (*proto.GetHistoryOrdersReply, error) {
	obj := proto.NewGetHistoryOrders(&proto.GetHistoryOrdersRequest{
		Date:   date,
		Market: market,
		Code:   makeCode6(code),
	})
	return client.execute(obj)
}

// GetHistoryTransactionData 查询指定交易日的历史分时逐笔成交；`date` 为交易日期，
// `start` 为起始偏移，`count` 为请求条数，`market` 取市场代码，`code` 为证券代码。
func (client *Client) GetHistoryTransactionData(date uint32, market uint8, code string, start uint16, count uint16) (*proto.GetHistoryTransactionDataReply, error) {
	obj := proto.NewGetHistoryTransactionData(&proto.GetHistoryTransactionDataRequest{
		Date:   date,
		Market: uint16(market),
		Code:   makeCode6(code),
		Start:  start,
		Count:  count,
	})
	return client.execute(obj)
}

// GetHistoryTransactionDataWithTrans 查询带买卖方向标识的历史分时逐笔成交；
// `date` 为交易日期，`start` 为起始偏移，`count` 为请求条数，`market` 取市场代码，`code` 为证券代码。
func (client *Client) GetHistoryTransactionDataWithTrans(date uint32, market uint8, code string, start uint16, count uint16) (*proto.GetHistoryTransactionDataWithTransReply, error) {
	obj := proto.NewGetHistoryTransactionDataWithTrans(&proto.GetHistoryTransactionDataRequest{
		Date:   date,
		Market: uint16(market),
		Code:   makeCode6(code),
		Start:  start,
		Count:  count,
	})
	return client.execute(obj)
}
