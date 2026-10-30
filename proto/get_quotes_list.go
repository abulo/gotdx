package proto

import (
	"bytes"
	"encoding/binary"
	"math"
)

// GetQuotesList 表示一次按分类与排序条件获取行情列表协议的完整流程对象。
//
// 该对象持有请求头、请求参数与响应数据；零值不可直接使用，必须通过
// NewGetQuotesList 构造。对象绑定单次查询流程，不支持多协程并发复用。
type GetQuotesList struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetQuotesListRequest
	reply      *GetQuotesListReply
}

// GetQuotesListRequest 是 GetQuotesList 查询使用的请求参数。
type GetQuotesListRequest struct {
	Category    uint16 // 板块或市场分类。
	SortType    uint16 // 排序方式。
	Start       uint16 // 起始偏移。
	Count       uint16 // 请求条数。
	SortReverse uint16 // 是否倒序排序。
	Mode        uint16 // 协议模式位，缺失时默认 5。
	Filter      uint16 // 过滤条件。
	One         uint16 // 固定位，缺失时默认 1。
	Zero        uint16 // 保留字段。
}

// GetQuotesListReply 是 GetQuotesList 查询解析完成后的响应数据。
type GetQuotesListReply struct {
	Block uint16          // 区块或分组标识。
	Count uint16          // 返回条数。
	List  []QuoteListItem // 排序后的行情快照列表。
}

// QuoteListItem 表示列表中的一条行情快照，包含价格、量额、统计指标与买卖档位。
type QuoteListItem struct {
	Market        uint8   // 市场代码。
	Code          string  // 证券代码。
	Active1       uint16  // 活跃度字段 1，语义待确认。
	Active2       uint16  // 活跃度字段 2，语义待确认。
	Close         float64 // 最新价，单位为元。
	Price         float64 // 当前价，通常与最新价一致。
	PreClose      float64 // 昨收价，单位为元。
	Open          float64 // 今开价，单位为元。
	High          float64 // 最高价，单位为元。
	Low           float64 // 最低价，单位为元。
	ServerTime    string  // 服务端时间。
	NegPrice      float64 // 特殊价格字段，语义待确认。
	Vol           int     // 总成交量。
	CurVol        int     // 现量。
	Amount        float64 // 总成交额。
	InVol         int     // 内盘量。
	OutVol        int     // 外盘量。
	SAmount       int     // 上涨家数或卖出相关统计字段，语义待确认。
	OpenAmount    int     // 开盘金额。
	BidLevels     []Level // 买盘档位列表。
	AskLevels     []Level // 卖盘档位列表。
	Unknown       uint16  // 未确认字段，语义待确认。
	RiseSpeed     float64 // 涨速。
	ShortTurnover float64 // 短周期换手。
	Min2Amount    float64 // 两分钟成交额。
	OpeningRush   float64 // 开盘抢筹指标。
	VolRiseSpeed  float64 // 量涨速。
	Depth         float64 // 委比或深度指标，语义待确认。
	Turnover      float64 // 换手率，由高层接口尽力估算补齐。
}

// NewGetQuotesList 构造排序行情列表查询对象；`req` 为 `nil` 时使用空请求参数，返回的对象可直接调用 BuildRequest 与 ParseResponse。
func NewGetQuotesList(req *GetQuotesListRequest) *GetQuotesList {
	obj := new(GetQuotesList)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetQuotesListRequest)
	obj.reply = new(GetQuotesListReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_QUOTESLIST
	obj.request.Mode = 5
	obj.request.One = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetQuotesList) applyRequest(req *GetQuotesListRequest) {
	if req.Mode == 0 {
		req.Mode = 5
	}
	if req.One == 0 {
		req.One = 1
	}
	obj.request = req
}

// BuildRequest 按小端序将请求头与请求参数编码为完整协议报文；缓冲区写入失败时返回错误。
func (obj *GetQuotesList) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 20
	obj.reqHeader.PkgLen2 = 20

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, obj.reqHeader)
	err = binary.Write(buf, binary.LittleEndian, obj.request)
	return buf.Bytes(), err
}

// ParseResponse 解析服务端返回的排序行情列表并填充响应对象；解码失败时返回错误。
func (obj *GetQuotesList) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header

	// 响应头部 4 字节：前 2 字节区块标识，后 2 字节小端序条数。
	obj.reply.Block = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Count = binary.LittleEndian.Uint16(data[2:4])
	pos := 4

	for i := uint16(0); i < obj.reply.Count; i++ {
		// 共享解码器：返回解码后的条目与更新后的读取偏移。
		item, nextPos, err := parseQuoteListItem(data, pos)
		if err != nil {
			return err
		}
		pos = nextPos
		obj.reply.List = append(obj.reply.List, item)
	}

	return nil
}

// Response 返回已解析完成的排序行情列表响应数据；未调用 ParseResponse 时返回空响应对象。
func (obj *GetQuotesList) Response() *GetQuotesListReply {
	return obj.reply
}

// GetQuotes 表示一次按证券列表批量获取行情快照协议的完整流程对象。
//
// 该对象与 GetQuotesList 的响应结构和解码流程一致；区别仅在于请求侧不按分类
// 排序，而是直接传入一组市场与代码。零值不可直接使用，必须通过 NewGetQuotes
// 构造，对象绑定单次查询流程，不支持多协程并发复用。
type GetQuotes struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetQuotesRequest
	reply      *GetQuotesReply
}

// GetQuotesRequest 是 GetQuotes 批量行情查询使用的请求参数。
type GetQuotesRequest struct {
	Stocks []Stock // 待查询的证券列表。
}

// GetQuotesReply 是 GetQuotes 批量行情查询解析完成后的响应数据。
type GetQuotesReply struct {
	Block uint16          // 区块或分组标识。
	Count uint16          // 返回条数。
	List  []QuoteListItem // 批量行情快照列表。
}

// NewGetQuotes 构造批量行情查询对象；`req` 为 `nil` 时使用空请求参数，返回的对象可直接调用 BuildRequest 与 ParseResponse。
func NewGetQuotes(req *GetQuotesRequest) *GetQuotes {
	obj := new(GetQuotes)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetQuotesRequest)
	obj.reply = new(GetQuotesReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_QUOTES
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetQuotes) applyRequest(req *GetQuotesRequest) {
	obj.request = req
}

// BuildRequest 将待查询证券列表编码为协议报文；缓冲区写入失败时返回错误。
func (obj *GetQuotes) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 2 + 10 + uint16(len(obj.request.Stocks)*7)
	obj.reqHeader.PkgLen2 = 2 + 10 + uint16(len(obj.request.Stocks)*7)

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, obj.reqHeader); err != nil {
		return nil, err
	}
	// 固定模式字 5。
	if err := binary.Write(buf, binary.LittleEndian, uint16(5)); err != nil {
		return nil, err
	}
	// 6 字节协议保留区，整体补零。
	if _, err := buf.Write(make([]byte, 6)); err != nil {
		return nil, err
	}
	// 小端序写入待查询证券个数。
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(obj.request.Stocks))); err != nil {
		return nil, err
	}
	for _, stock := range obj.request.Stocks {
		code := make([]byte, 6)
		copy(code, stock.Code)
		if err := binary.Write(buf, binary.LittleEndian, stock.Market); err != nil {
			return nil, err
		}
		if _, err := buf.Write(code); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// ParseResponse 解析服务端返回的批量行情列表并填充响应对象；解码失败时返回错误。
func (obj *GetQuotes) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header

	obj.reply.Block = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Count = binary.LittleEndian.Uint16(data[2:4])
	pos := 4

	for i := uint16(0); i < obj.reply.Count; i++ {
		item, nextPos, err := parseQuoteListItem(data, pos)
		if err != nil {
			return err
		}
		pos = nextPos
		obj.reply.List = append(obj.reply.List, item)
	}

	return nil
}

// Response 返回已解析完成的批量行情响应数据；未调用 ParseResponse 时返回空响应对象。
func (obj *GetQuotes) Response() *GetQuotesReply {
	return obj.reply
}

// parseQuoteListItem 解码一条排序或批量行情快照，返回条目与下一条记录的读取偏移。
//
// 单条记录按固定顺序解码：定长头部之后是基准价与各价格差值、量额字段与一档买卖，
// 末尾为速度、换手等统计字段与多段保留区；价格以基准价加差值还原后除以 100 得到元。
func parseQuoteListItem(data []byte, pos int) (QuoteListItem, int, error) {
	item := QuoteListItem{}

	// 市场 1 字节。
	item.Market = data[pos]
	pos++
	// 代码 6 字节，按 `GBK` 还原字符串。
	item.Code = Utf8ToGbk(data[pos : pos+6])
	pos += 6
	// 活跃度字段 1，2 字节小端序。
	item.Active1 = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2

	// 基准价与各价格差值；后续价位为基准价加差值。
	basePrice := getprice(data, &pos)
	preCloseDiff := getprice(data, &pos)
	openDiff := getprice(data, &pos)
	highDiff := getprice(data, &pos)
	lowDiff := getprice(data, &pos)
	serverTimeRaw := getprice(data, &pos)
	negPriceRaw := getprice(data, &pos)
	item.Vol = getprice(data, &pos)
	item.CurVol = getprice(data, &pos)
	// 成交额为 4 字节小端序单精度浮点数。
	item.Amount = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4])))
	pos += 4
	item.InVol = getprice(data, &pos)
	item.OutVol = getprice(data, &pos)
	item.SAmount = getprice(data, &pos)
	item.OpenAmount = getprice(data, &pos)

	// 一档买卖：价格相对基准价偏移，档位量独立读取。
	bid := getprice(data, &pos) + basePrice
	ask := getprice(data, &pos) + basePrice
	bidVol := getprice(data, &pos)
	askVol := getprice(data, &pos)
	item.BidLevels = append(item.BidLevels, Level{Price: float64(bid) / 100.0, Vol: bidVol})
	item.AskLevels = append(item.AskLevels, Level{Price: float64(ask) / 100.0, Vol: askVol})

	// 尾部统计区：未知 2 字节小端序字段。
	item.Unknown = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2
	// 涨速：小端序无符号整数转有符号短整数。
	riseSpeed := int16(binary.LittleEndian.Uint16(data[pos : pos+2]))
	pos += 2
	shortTurnover := int16(binary.LittleEndian.Uint16(data[pos : pos+2]))
	pos += 2
	// 两分钟成交额，4 字节小端序单精度浮点数。
	min2Amount := math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	openingRush := int16(binary.LittleEndian.Uint16(data[pos : pos+2]))
	pos += 2
	// 10 字节协议保留区，整体跳过。
	pos += 10
	volRiseSpeed := math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	depth := math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	// 24 字节协议保留区，整体跳过。
	pos += 24
	item.Active2 = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2

	// 基准价加差值还原各价位，除以 100 得到元。
	item.Close = float64(basePrice) / 100.0
	item.Price = item.Close
	item.PreClose = float64(basePrice+preCloseDiff) / 100.0
	item.Open = float64(basePrice+openDiff) / 100.0
	item.High = float64(basePrice+highDiff) / 100.0
	item.Low = float64(basePrice+lowDiff) / 100.0
	item.ServerTime = formatServerTime(serverTimeRaw)
	item.NegPrice = float64(negPriceRaw) / 100.0
	// 速度与换手类指标为放大 100 倍的有符号整数，解析后除以 100。
	item.RiseSpeed = float64(riseSpeed) / 100.0
	item.ShortTurnover = float64(shortTurnover) / 100.0
	item.Min2Amount = float64(min2Amount)
	item.OpeningRush = float64(openingRush) / 100.0
	item.VolRiseSpeed = float64(volRiseSpeed)
	item.Depth = float64(depth)

	return item, pos, nil
}
