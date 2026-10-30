package gotdx

import (
	"math"
	"time"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"github.com/spf13/cast"
)

// quotationClient 返回用于主站行情请求的已连接客户端。
// 主站模式直接返回自身；扩展站/MAC 模式惰性创建内嵌主站连接并按需握手，保证统一接口背后始终是主站通道。
func (client *Client) quotationClient() (*Client, error) {
	if client.mode == clientModeMain {
		// 自身尚未连接时先完成主站握手。
		if client.conn == nil {
			if _, err := client.Connect(); err != nil {
				return nil, err
			}
		}
		return client, nil
	}

	client.mu.Lock()
	if client.main == nil {
		client.main = newClientWithOptions(client.opt, clientModeMain)
	}
	main := client.main
	client.mu.Unlock()

	if main.conn == nil {
		if _, err := main.Connect(); err != nil {
			return nil, err
		}
	}
	return main, nil
}

// exQuotationClient 返回用于扩展市场请求的已连接客户端。
// 扩展站模式直接返回自身；其他模式惰性创建内嵌扩展站连接并按需登录。
func (client *Client) exQuotationClient() (*Client, error) {
	if client.mode == clientModeEx {
		// 自身尚未连接时先完成扩展站登录握手。
		if client.conn == nil {
			if _, err := client.ConnectEx(); err != nil {
				return nil, err
			}
		}
		return client, nil
	}

	client.mu.Lock()
	if client.ex == nil {
		client.ex = newClientWithOptions(client.opt, clientModeEx)
	}
	ex := client.ex
	client.mu.Unlock()

	if ex.conn == nil {
		if _, err := ex.ConnectEx(); err != nil {
			return nil, err
		}
	}
	return ex, nil
}

// StockCount 获取指定市场内的证券数目。
func (client *Client) StockCount(market uint8) (uint16, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return 0, err
	}
	reply, err := qc.GetSecurityCount(market)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

// StockList 分页获取指定市场内的证券代码与名称列表。
func (client *Client) StockList(market uint8, start uint32, count uint32) ([]proto.Security, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetSecurityListRange(market, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockAll 自动分页拉取指定市场内的全部证券列表。
func (client *Client) StockAll(market uint8) ([]proto.Security, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply := &proto.GetSecurityListReply{}

	size := uint32(1000)
	for start := uint32(0); ; start += size {
		resp, err := qc.GetSecurityListRange(market, start, size)
		if err != nil {
			return nil, err
		}
		reply.Count += resp.Count
		reply.List = append(reply.List, resp.List...)
		// 本页返回条数不足分页大小，说明已是最后一页，停止翻页。
		if resp.Count < uint16(size) {
			break
		}
	}
	return reply.List, nil
}

// StockListOld 使用旧版协议获取市场内证券列表。
func (client *Client) StockListOld(market uint8, start uint16) ([]proto.Security, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetSecurityListOld(market, start)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockFeature452 获取证券扩展特征信息（试验协议 `0x452`）。
func (client *Client) StockFeature452(start uint32, count uint32) ([]proto.SecurityFeature452Item, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetSecurityFeature452(start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockKLine 获取证券 K 线，自动修正 9:30 分时数据并按流通股本补齐换手率。
func (client *Client) StockKLine(category uint16, market uint8, code string, start uint16, count uint16, times uint16, adjust uint16) ([]proto.SecurityBar, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetKLine(category, market, code, start, count, times, adjust)
	if err != nil {
		return nil, err
	}
	client.StockKLine930(category, market, code, reply)
	applyTurnoverToBars(reply.List, client.loadFloatShares(qc, market, code))
	return reply.List, nil
}

// StockKLine930 补齐 1 分钟分时中缺失的 09:30 首根 K 线；仅对 1 分钟与扩展 1 分钟周期生效。
//
// 服务器通常从 09:31 开始返回分钟线，09:30 集合竞价首笔成交缺失；本方法依据当日或历史首笔
// 逐笔成交重建该首根线，其开、高、低、收均为首笔成交价。
func (client *Client) StockKLine930(category uint16, market uint8, code string, reply *proto.GetSecurityBarsReply) {
	if category == types.KLINE_TYPE_EXHQ_1MIN || category == types.KLINE_TYPE_1MIN {
		// 取服务器当前交易日期，用于区分当前 K 线属于当日还是历史日期。
		if info, err := client.GetServerHeartbeat(); err == nil {
			list := make([]proto.SecurityBar, 0)
			for _, v := range reply.List {
				if v.DateTime.Format(time.TimeOnly) == "09:31:00" {
					data := proto.SecurityBar{}
					if cast.ToUint32(v.DateTime.Format("20060102")) == info.Date {
						// 当日：拉取当日全部逐笔成交，取首笔作为 09:30 线价格。
						if trans, err := client.StockFullTransaction(market, code); err == nil {
							data.Open = trans[0].Price
							data.High = trans[0].Price
							data.Low = trans[0].Price
							data.Close = trans[0].Price
							data.PreClose = v.PreClose
							data.Vol = cast.ToFloat64(trans[0].Vol)
							data.Amount = cast.ToFloat64(trans[0].Vol) * trans[0].Price * 100
							data.DateTime = time.Date(v.DateTime.Year(), v.DateTime.Month(), v.DateTime.Day(), 9, 30, 0, 0, v.DateTime.Location())
							v.PreClose = trans[0].Price
							data.RiseRate = data.GetRiseRate()
							data.RisePrice = data.GetRisePrice()
							list = append(list, data)
						}
					} else {
						// 历史日期：拉取当日历史逐笔成交重建 09:30 线。
						if trans, err := client.StockHistoryFullTransaction(cast.ToUint32(v.DateTime.Format("20060102")), market, code); err == nil {
							data.Open = trans[0].Price
							data.High = trans[0].Price
							data.Low = trans[0].Price
							data.Close = trans[0].Price
							data.PreClose = v.PreClose
							data.Vol = cast.ToFloat64(trans[0].Vol)
							data.Amount = cast.ToFloat64(trans[0].Vol) * trans[0].Price * 100
							data.DateTime = time.Date(v.DateTime.Year(), v.DateTime.Month(), v.DateTime.Day(), 9, 30, 0, 0, v.DateTime.Location())
							v.PreClose = trans[0].Price
							data.RiseRate = data.GetRiseRate()
							data.RisePrice = data.GetRisePrice()
							list = append(list, data)
						}
					}
				}
				list = append(list, v)
			}
			reply.List = list
			reply.Count = uint16(len(list))
		}
	}
}

// StockFullKLine 自动向前翻页拉取 K 线，直到回调 f 命中某根 K 线或历史取完为止。
func (client *Client) StockFullKLine(category uint16, market uint8, code string, times uint16, adjust uint16, f func(kline proto.SecurityBar) bool) ([]proto.SecurityBar, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply := &proto.GetSecurityBarsReply{}
	count := uint16(20)
	for start := uint16(0); ; start += count {
		r, err := qc.GetKLine(category, market, code, start, count, times, adjust)
		if err != nil {
			return nil, err
		}
		// 从最新一根向历史方向回扫，回调命中即保留命中位置及之前的全部 K 线后返回。
		for i := len(r.List) - 1; i >= 0; i-- {
			if f(r.List[i]) {
				reply.Count += r.Count - uint16(i)
				reply.List = append(r.List[i:], reply.List...)
				applyTurnoverToBars(reply.List, client.loadFloatShares(qc, market, code))
				return reply.List, nil
			}
		}
		// 未命中：把本页 K 线拼到已收集序列前面，继续向前翻页。
		reply.Count += r.Count
		reply.List = append(r.List, reply.List...)
		// 本页不足分页大小说明历史已取完，结束翻页。
		if r.Count < count {
			break
		}
	}
	client.StockKLine930(category, market, code, reply)
	applyTurnoverToBars(reply.List, client.loadFloatShares(qc, market, code))
	return reply.List, nil
}

// StockKLineOffset 使用偏移协议获取证券 K 线，并补齐换手率。
func (client *Client) StockKLineOffset(category uint16, market uint8, code string, start uint16, count uint16, times uint16, adjust uint16) ([]proto.SecurityBar, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetSecurityBarsOffset(category, market, code, start, count, times, adjust)
	if err != nil {
		return nil, err
	}
	client.StockKLine930(category, market, code, reply)
	applyTurnoverToBars(reply.List, client.loadFloatShares(qc, market, code))
	return reply.List, nil
}

// StockTickChart 获取当日分时图数据。
func (client *Client) StockTickChart(market uint8, code string, start uint16, count uint16) ([]proto.MinuteTimeData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetTickChart(market, code, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockHistoryTickChart 获取指定日期的历史分时图数据。
func (client *Client) StockHistoryTickChart(date uint32, market uint8, code string) ([]proto.HistoryMinuteTimeData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetHistoryTickChart(date, market, code)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockIndexInfo 获取指数概况。
func (client *Client) StockIndexInfo(market uint8, code string) (*proto.GetIndexInfoReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetIndexInfo(market, code)
}

// StockIndexMomentum 获取指数动量序列。
func (client *Client) StockIndexMomentum(market uint8, code string) ([]int, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetIndexMomentum(market, code)
	if err != nil {
		return nil, err
	}
	return reply.Values, nil
}

// StockChartSampling 获取分时缩略采样价格。
func (client *Client) StockChartSampling(market uint8, code string) ([]float64, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetChartSampling(market, code)
	if err != nil {
		return nil, err
	}
	return reply.Prices, nil
}

// StockQuotesDetail 批量获取详细盘口行情，自动修正小数位并按流通股本补齐换手率。
func (client *Client) StockQuotesDetail(markets []uint8, codes []string) ([]proto.SecurityQuote, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetQuotesDetail(markets, codes)
	if err != nil {
		return nil, err
	}
	keys := stockKeysFromPairLists(markets, codes)
	// 先按小数位数换算价格，再按流通股本回填换手率。
	applyDecimalPointToSecurityQuotes(reply.List, client.loadDecimalPointMap(qc, keys))
	applyTurnoverToSecurityQuotes(reply.List, client.loadFloatSharesMap(qc, keys))
	return reply.List, nil
}

// StockQuotesList 获取排序后的行情列表，自动修正小数位并补齐换手率。
func (client *Client) StockQuotesList(category uint8, start uint16, count uint16, sortType uint16, reverse bool, filter uint16) ([]proto.QuoteListItem, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetQuotesList(category, start, count, sortType, reverse, filter)
	if err != nil {
		return nil, err
	}
	keys := stockKeysFromQuoteItems(reply.List)
	// 先按小数位数换算价格，再按流通股本回填换手率。
	applyDecimalPointToQuoteList(reply.List, client.loadDecimalPointMap(qc, keys))
	applyTurnoverToQuoteList(reply.List, client.loadFloatSharesMap(qc, keys))
	return reply.List, nil
}

// StockQuotes 批量获取行情快照列表，自动修正小数位并补齐换手率。
func (client *Client) StockQuotes(markets []uint8, codes []string) ([]proto.QuoteListItem, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetQuotes(markets, codes)
	if err != nil {
		return nil, err
	}
	keys := stockKeysFromQuoteItems(reply.List)
	// 先按小数位数换算价格，再按流通股本回填换手率。
	applyDecimalPointToQuoteList(reply.List, client.loadDecimalPointMap(qc, keys))
	applyTurnoverToQuoteList(reply.List, client.loadFloatSharesMap(qc, keys))
	return reply.List, nil
}

// StockQuotesEncrypt 获取加密行情报价列表。
func (client *Client) StockQuotesEncrypt(markets []uint8, codes []string) ([]proto.EncryptedQuoteItem, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetQuotesEncrypt(markets, codes)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockAuction 获取集合竞价数据。
func (client *Client) StockAuction(market uint8, code string, start uint32, count uint32) ([]proto.AuctionData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetAuction(market, code, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockFullAuction 自动分页拉取集合竞价全部数据。
func (client *Client) StockFullAuction(market uint8, code string) ([]proto.AuctionData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply := &proto.GetAuctionReply{}
	count := uint32(50)
	for start := uint32(0); ; start += count {
		res, err := qc.GetAuction(market, code, start, count)
		if err != nil {
			return nil, err
		}
		reply.Count += res.Count
		// 新取到的一页排在已有序列之前，保持时间顺序不反转。
		reply.List = append(res.List, reply.List...)
		if res.Count < uint16(count) {
			break
		}
	}
	return reply.List, nil
}

// StockTopBoard 获取主行情排行榜。
func (client *Client) StockTopBoard(category uint8, size uint8) (*proto.GetTopBoardReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetTopBoard(category, size)
}

// StockUnusual 获取主力监控异动数据。
func (client *Client) StockUnusual(market uint8, start uint32, count uint32) ([]proto.UnusualData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetUnusual(market, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockVolumeProfile 获取成交分布与盘口快照。
func (client *Client) StockVolumeProfile(market uint8, code string) (*proto.GetVolumeProfileReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetVolumeProfile(market, code)
	if err != nil {
		return nil, err
	}
	applyTurnoverToVolumeProfile(reply, client.loadFloatShares(qc, market, code))
	return reply, nil
}

// StockTransaction 获取当日分时成交明细。
func (client *Client) StockTransaction(market uint8, code string, start uint16, count uint16) ([]proto.TransactionData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetTransactionData(market, code, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockFullTransaction 自动分页拉取当日全部分时成交明细。
func (client *Client) StockFullTransaction(market uint8, code string) ([]proto.TransactionData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply := &proto.GetTransactionDataReply{}
	count := uint16(600)
	for start := uint16(0); ; start += count {
		res, err := qc.GetTransactionData(market, code, start, count)
		if err != nil {
			return nil, err
		}
		reply.Count += res.Count
		reply.List = append(res.List, reply.List...)
		if res.Count < count { // 说明已经获取完了
			break
		}
	}
	return reply.List, nil
}

// StockHistoryOrders 获取历史委托分布。
func (client *Client) StockHistoryOrders(date uint32, market uint8, code string) ([]proto.HistoryOrderData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetHistoryOrders(date, market, code)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockHistoryTransaction 获取指定日期的历史分时成交明细。
func (client *Client) StockHistoryTransaction(date uint32, market uint8, code string, start uint16, count uint16) ([]proto.HistoryTransactionData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetHistoryTransactionData(date, market, code, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockHistoryFullTransaction 自动分页拉取指定日期的全部分时成交明细。
func (client *Client) StockHistoryFullTransaction(date uint32, market uint8, code string) ([]proto.HistoryTransactionData, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply := &proto.GetHistoryTransactionDataReply{}
	count := uint16(600)
	for start := uint16(0); ; start += count {
		res, err := qc.GetHistoryTransactionData(date, market, code, start, count)
		if err != nil {
			return nil, err
		}
		reply.Count += res.Count
		reply.List = append(res.List, reply.List...)
		// 本页不足分页大小即为最后一页，停止翻页。
		if res.Count < count {
			break
		}
	}
	return reply.List, nil
}

// StockHistoryTransactionWithTrans 获取带买卖方向的历史分时成交明细。
func (client *Client) StockHistoryTransactionWithTrans(date uint32, market uint8, code string, start uint16, count uint16) ([]proto.HistoryTransactionDataWithTrans, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := qc.GetHistoryTransactionDataWithTrans(date, market, code, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// StockHistoryFullTransactionWithTrans 自动分页拉取带买卖方向的历史全部分时成交明细。
func (client *Client) StockHistoryFullTransactionWithTrans(date uint32, market uint8, code string) ([]proto.HistoryTransactionDataWithTrans, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	reply := &proto.GetHistoryTransactionDataWithTransReply{}
	count := uint16(600)
	for start := uint16(0); ; start += count {
		res, err := qc.GetHistoryTransactionDataWithTrans(date, market, code, start, count)
		if err != nil {
			return nil, err
		}
		reply.Count += res.Count
		reply.List = append(res.List, reply.List...)
		// 本页不足分页大小即为最后一页，停止翻页。
		if res.Count < count {
			break
		}
	}
	return reply.List, nil
}

// StockF10 获取公司 F10 信息聚合结果（章节、除权除息与财务信息）。
func (client *Client) StockF10(market uint8, code string) (*CompanyInfoBundle, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetCompanyInfo(market, code)
}

// StockBlock 下载并解析指定板块文件，返回扁平化后的板块条目列表。
func (client *Client) StockBlock(filename string) ([]BlockFlatItem, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetParsedBlockFile(filename)
}

// MainTodoB 获取主站试验协议 `0x000b` 的原始响应。
func (client *Client) MainTodoB() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetTodoB()
}

// MainTodoFDE 获取主站试验协议 `0x0fde` 的原始响应。
func (client *Client) MainTodoFDE() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetTodoFDE()
}

// MainClient264B 获取客户端信息协议 `0x264b` 的原始响应。
func (client *Client) MainClient264B() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetClient264B()
}

// MainClient26AC 获取客户端信息协议 `0x26ac` 的原始响应。
func (client *Client) MainClient26AC() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetClient26AC()
}

// MainClient26AD 获取客户端信息协议 `0x26ad` 的原始响应。
func (client *Client) MainClient26AD() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetClient26AD()
}

// MainClient26AE 获取客户端信息协议 `0x26ae` 的原始响应。
func (client *Client) MainClient26AE() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetClient26AE()
}

// MainClient26B1 获取客户端信息协议 `0x26b1` 的原始响应。
func (client *Client) MainClient26B1() (*proto.RawDataReply, error) {
	qc, err := client.quotationClient()
	if err != nil {
		return nil, err
	}
	return qc.GetClient26B1()
}

// ExCount 获取扩展市场内的标的总数。
func (client *Client) ExCount() (uint32, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return 0, err
	}
	reply, err := eqc.ExGetCount()
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

// ExCategoryList 获取扩展市场分类（港股、美股、期货等）列表。
func (client *Client) ExCategoryList() ([]proto.ExCategoryItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetCategoryList()
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExList 分页获取扩展市场证券列表。
func (client *Client) ExList(start uint32, count uint16) ([]proto.ExListItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetList(start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExListExtra 获取扩展市场附加列表记录；参数 a、b 的确切语义待确认。
func (client *Client) ExListExtra(a uint16, b uint16, count uint16) ([]proto.ExExtraListItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetListExtra(a, b, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExQuotesList 分页获取扩展市场排序行情列表。
func (client *Client) ExQuotesList(category uint8, start uint16, count uint16, sortType uint16, reverse bool) ([]proto.ExQuoteItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetQuotesList(category, start, count, sortType, reverse)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExQuote 获取单只扩展证券的行情报价。
func (client *Client) ExQuote(category uint8, code string) (*proto.ExQuoteItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetQuote(category, code)
	if err != nil {
		return nil, err
	}
	return &reply.Item, nil
}

// ExQuotes 批量获取扩展市场行情报价。
func (client *Client) ExQuotes(categories []uint8, codes []string) ([]proto.ExQuoteItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetQuotes(categories, codes)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExQuotes2 使用第二套批量报价协议获取扩展市场行情报价。
func (client *Client) ExQuotes2(categories []uint8, codes []string) ([]proto.ExQuoteItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetQuotes2(categories, codes)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExKLine 获取扩展市场证券 K 线数据。
func (client *Client) ExKLine(category uint8, code string, period uint16, start uint32, count uint16, times uint16) ([]proto.ExKLineItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetKLine(category, code, period, start, count, times)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExExperiment2487 获取试验协议 `0x2487` 的单品种行情原始响应。
func (client *Client) ExExperiment2487(category uint8, code string) (*proto.ExExperiment2487Reply, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	return eqc.ExGetExperiment2487(category, code)
}

// ExExperiment2488 获取试验协议 `0x2488` 的数据记录；参数 `mode` 与各字段含义语义待确认。
func (client *Client) ExExperiment2488(category uint8, code string, mode uint16) ([]proto.ExExperiment2488Item, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetExperiment2488(category, code, mode)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExKLine2 使用第二套 K 线协议获取扩展市场证券 K 线数据。
func (client *Client) ExKLine2(category uint8, code string, period uint16, start uint32, count uint32, times uint16) ([]proto.ExKLineItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetKLine2(category, code, period, start, count, times)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExHistoryTransaction 获取扩展市场指定日期的历史成交明细。
func (client *Client) ExHistoryTransaction(date uint32, category uint8, code string) ([]proto.ExHistoryTransactionItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetHistoryTransaction(date, category, code)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExTickChart 获取扩展市场分时图；参数 `date` 为 0 时取当日分时，否则取指定历史日期分时。
func (client *Client) ExTickChart(category uint8, code string, date uint32) ([]proto.ExTickChartData, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	if date == 0 {
		// `date` 为 0 表示取当日分时，否则走历史分时协议。
		reply, err := eqc.ExGetTickChart(category, code)
		if err != nil {
			return nil, err
		}
		return reply.List, nil
	}
	reply, err := eqc.ExGetHistoryTickChart(date, category, code)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExChartSampling 获取扩展市场分时缩略采样价格序列。
func (client *Client) ExChartSampling(category uint8, code string) ([]float64, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetChartSampling(category, code)
	if err != nil {
		return nil, err
	}
	return reply.Prices, nil
}

// ExBoardList 分页获取扩展市场板块列表。
func (client *Client) ExBoardList(boardType uint16, start uint16, pageSize uint16) ([]proto.ExBoardListItem, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetBoardList(boardType, start, pageSize)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExMapping2562 获取试验协议 `0x2562` 的映射记录；多数字段含义语义待确认。
func (client *Client) ExMapping2562(market uint16, start uint32, count uint32) ([]proto.ExMapping2562Item, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return nil, err
	}
	reply, err := eqc.ExGetMapping2562(market, start, count)
	if err != nil {
		return nil, err
	}
	return reply.List, nil
}

// ExTable 获取扩展市场表格文件的文本内容。
func (client *Client) ExTable() (string, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return "", err
	}
	return eqc.ExGetTable()
}

// ExTableDetail 获取扩展市场表格明细文件的文本内容。
func (client *Client) ExTableDetail() (string, error) {
	eqc, err := client.exQuotationClient()
	if err != nil {
		return "", err
	}
	return eqc.ExGetTableDetail()
}

type stockKey struct {
	Market uint8
	Code   string
}

// stockKeysFromPairLists 把市场与代码配对成查询键；市场列表较短时按短列表截断，防止越界。
func stockKeysFromPairLists(markets []uint8, codes []string) []stockKey {
	items := make([]stockKey, 0, len(codes))
	for i, code := range codes {
		if i >= len(markets) {
			break
		}
		items = append(items, stockKey{Market: markets[i], Code: code})
	}
	return items
}

// stockKeysFromQuoteItems 从行情列表中提取市场/代码查询键。
func stockKeysFromQuoteItems(items []proto.QuoteListItem) []stockKey {
	keys := make([]stockKey, 0, len(items))
	for _, item := range items {
		keys = append(keys, stockKey{Market: item.Market, Code: item.Code})
	}
	return keys
}

// loadDecimalPointMap 批量获取一组证券的价格小数位数。
// 流程：先命中已有缓存；未命中且市场尚未整市场加载时登记该市场，统一整市场分页拉取后再回填缓存。
func (client *Client) loadDecimalPointMap(qc *Client, keys []stockKey) map[stockKey]int8 {
	out := make(map[stockKey]int8, len(keys))
	markets := make(map[uint8]struct{})
	for _, key := range keys {
		if key.Code == "" {
			continue
		}
		if decimal, ok := client.cachedDecimalPoint(key); ok {
			out[key] = decimal
			continue
		}
		// 同一市场只登记一次，避免整市场重复拉取。
		if !client.decimalPointMarketLoaded(key.Market) {
			markets[key.Market] = struct{}{}
		}
	}

	// 整市场补齐缓存后再做第二轮回填。
	for market := range markets {
		client.loadDecimalPointMarket(qc, market)
	}

	for _, key := range keys {
		if key.Code == "" {
			continue
		}
		if _, ok := out[key]; ok {
			continue
		}
		if decimal, ok := client.cachedDecimalPoint(key); ok {
			out[key] = decimal
		}
	}
	return out
}

// cachedDecimalPoint 在锁内读取单只证券已缓存的小数位数。
func (client *Client) cachedDecimalPoint(key stockKey) (int8, bool) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.decimalPoints == nil {
		return 0, false
	}
	codes := client.decimalPoints[key.Market]
	if codes == nil {
		return 0, false
	}
	decimal, ok := codes[key.Code]
	return decimal, ok
}

// decimalPointMarketLoaded 判断某市场的证券小数位数是否已整市场加载过。
func (client *Client) decimalPointMarketLoaded(market uint8) bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.decimalPointsLoaded != nil && client.decimalPointsLoaded[market]
}

// loadDecimalPointMarket 整市场分页拉取证券列表并缓存小数位数；已加载过直接返回，
// 任一页请求出错即中止该市场加载，空页或不足一页判定为最后一页。
func (client *Client) loadDecimalPointMarket(qc *Client, market uint8) {
	if client.decimalPointMarketLoaded(market) {
		return
	}

	pageSize := uint32(types.DefaultSecurityListCount)
	for start := uint32(0); ; start += pageSize {
		reply, err := qc.GetSecurityListRange(market, start, pageSize)
		if err != nil {
			return
		}
		client.storeDecimalPointItems(market, reply.List)
		// 空页或不足分页大小说明市场已遍历完，结束翻页。
		if len(reply.List) == 0 || uint32(len(reply.List)) < pageSize {
			break
		}
	}

	client.mu.Lock()
	if client.decimalPointsLoaded == nil {
		client.decimalPointsLoaded = make(map[uint8]bool)
	}
	client.decimalPointsLoaded[market] = true
	client.mu.Unlock()
}

// storeDecimalPointItems 在锁内把一页证券的小数位数写入市场缓存，跳过空代码。
func (client *Client) storeDecimalPointItems(market uint8, items []proto.Security) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.decimalPoints == nil {
		client.decimalPoints = make(map[uint8]map[string]int8)
	}
	if client.decimalPoints[market] == nil {
		client.decimalPoints[market] = make(map[string]int8, len(items))
	}
	for _, item := range items {
		if item.Code == "" {
			continue
		}
		client.decimalPoints[market][item.Code] = item.DecimalPoint
	}
}

// applyDecimalPointToSecurityQuotes 按缓存小数位数把详细盘口的价格字段换算为元；
// 未取到位数或换算系数为 1 时保持原值。
func applyDecimalPointToSecurityQuotes(items []proto.SecurityQuote, decimals map[stockKey]int8) {
	for i := range items {
		decimal, ok := decimals[stockKey{Market: items[i].Market, Code: items[i].Code}]
		if !ok {
			continue
		}
		factor := decimalPriceFactor(decimal, 100)
		if factor == 1 {
			continue
		}
		items[i].Close *= factor
		items[i].Price *= factor
		items[i].PreClose *= factor
		items[i].LastClose *= factor
		items[i].Open *= factor
		items[i].High *= factor
		items[i].Low *= factor
		items[i].NegPrice *= factor
		for j := range items[i].BidLevels {
			items[i].BidLevels[j].Price *= factor
		}
		for j := range items[i].AskLevels {
			items[i].AskLevels[j].Price *= factor
		}
		items[i].Bid1 *= factor
		items[i].Bid2 *= factor
		items[i].Bid3 *= factor
		items[i].Bid4 *= factor
		items[i].Bid5 *= factor
		items[i].Ask1 *= factor
		items[i].Ask2 *= factor
		items[i].Ask3 *= factor
		items[i].Ask4 *= factor
		items[i].Ask5 *= factor
	}
}

// applyDecimalPointToQuoteList 按缓存小数位数把行情快照的价格字段换算为元。
func applyDecimalPointToQuoteList(items []proto.QuoteListItem, decimals map[stockKey]int8) {
	for i := range items {
		decimal, ok := decimals[stockKey{Market: items[i].Market, Code: items[i].Code}]
		if !ok {
			continue
		}
		factor := decimalPriceFactor(decimal, 100)
		if factor == 1 {
			continue
		}
		items[i].Close *= factor
		items[i].Price *= factor
		items[i].PreClose *= factor
		items[i].Open *= factor
		items[i].High *= factor
		items[i].Low *= factor
		items[i].NegPrice *= factor
		for j := range items[i].BidLevels {
			items[i].BidLevels[j].Price *= factor
		}
		for j := range items[i].AskLevels {
			items[i].AskLevels[j].Price *= factor
		}
	}
}

// decimalPriceFactor 把协议隐含的当前除数按目标小数位数换算成价格乘数；
// 小数位数越界（0~8 以外）视为非法，返回 1 不做调整。
func decimalPriceFactor(decimal int8, currentDivisor float64) float64 {
	if decimal < 0 || decimal > 8 {
		return 1
	}
	divisor := math.Pow10(int(decimal))
	if divisor == 0 {
		return 1
	}
	return currentDivisor / divisor
}

// loadFloatShares 获取单只证券的流通股本；空代码直接返回 0。
func (client *Client) loadFloatShares(qc *Client, market uint8, code string) float64 {
	if code == "" {
		return 0
	}
	return client.loadFloatSharesMap(qc, []stockKey{{Market: market, Code: code}})[stockKey{Market: market, Code: code}]
}

// loadFloatSharesMap 批量获取流通股本：对重复键去重，财务信息拉取失败或流通股本非正时跳过。
func (client *Client) loadFloatSharesMap(qc *Client, keys []stockKey) map[stockKey]float64 {
	out := make(map[stockKey]float64, len(keys))
	seen := make(map[stockKey]struct{}, len(keys))
	for _, key := range keys {
		if key.Code == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		finance, err := qc.GetFinanceInfo(key.Market, key.Code)
		if err != nil {
			continue
		}
		if finance == nil || finance.FloatShares <= 0 {
			continue
		}
		out[key] = float64(finance.FloatShares)
	}
	return out
}

// applyTurnoverToSecurityQuotes 用流通股本回填详细盘口换手率；股本缺失或非正的证券跳过。
func applyTurnoverToSecurityQuotes(items []proto.SecurityQuote, shares map[stockKey]float64) {
	for i := range items {
		if floatShares := shares[stockKey{Market: items[i].Market, Code: items[i].Code}]; floatShares > 0 {
			items[i].Turnover = round2(float64(items[i].Vol) * 10000 / floatShares)
		}
	}
}

// applyTurnoverToQuoteList 用流通股本回填行情快照换手率。
func applyTurnoverToQuoteList(items []proto.QuoteListItem, shares map[stockKey]float64) {
	for i := range items {
		if floatShares := shares[stockKey{Market: items[i].Market, Code: items[i].Code}]; floatShares > 0 {
			items[i].Turnover = round2(float64(items[i].Vol) * 10000 / floatShares)
		}
	}
}

// applyTurnoverToBars 用单只证券流通股本回填 K 线换手率；股本非正时不处理。
func applyTurnoverToBars(items []proto.SecurityBar, floatShares float64) {
	if floatShares <= 0 {
		return
	}
	for i := range items {
		items[i].Turnover = round2(items[i].Vol * 100 / floatShares)
	}
}

// applyTurnoverToVolumeProfile 用流通股本回填成交分布的总换手率。
func applyTurnoverToVolumeProfile(reply *proto.GetVolumeProfileReply, floatShares float64) {
	if reply == nil || floatShares <= 0 {
		return
	}
	reply.Turnover = round2(float64(reply.Vol) * 10000 / floatShares)
}

// round2 把数值四舍五入保留两位小数。
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
