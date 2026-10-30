package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// ExGetKLine 表示扩展市场 K 线查询协议，串联请求构造、报文编码、响应解码与结果获取。
//
// 响应中首根 K 线仅用于取得昨收基准，不进入最终列表；每根 K 线的涨跌价与涨跌幅
// 由本根收盘与上一根收盘滚动计算。实例必须通过 NewExGetKLine 构造，零值不可用。
type ExGetKLine struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *ExGetKLineRequest
	reply      *ExGetKLineReply
}

// ExGetKLineRequest 保存 K 线查询的市场类别、代码、周期与起始偏移等参数。
type ExGetKLineRequest struct {
	Category uint8   // 市场类别。
	Code     [9]byte // 证券代码。
	Period   uint16  // K 线周期类别。
	Times    uint16  // 周期倍数。
	Start    uint32  // 起始偏移。
	Count    uint16  // 请求条数。
}

// ExGetKLineReply 保存解码完成的 K 线查询响应结果。
type ExGetKLineReply struct {
	Category uint8         // 市场类别。
	Name     string        // 证券名称。
	Period   uint16        // K 线周期类别。
	Times    uint16        // 周期倍数。
	Start    uint32        // 起始偏移。
	Count    uint16        // 返回条数。
	List     []ExKLineItem // K 线数据列表。
}

// ExKLineItem 表示扩展市场的一根 K 线，包含时间、开高低收、成交量额及涨跌信息。
type ExKLineItem struct {
	DateTime  time.Time // K 线时间。
	Open      float64   // 开盘价。
	High      float64   // 最高价。
	Low       float64   // 最低价。
	Close     float64   // 收盘价。
	Amount    float64   // 成交额。
	Vol       uint32    // 成交量。
	PreClose  float64   // 昨收价。
	LastClose float64   // 前收价。
	RisePrice float64   // 涨跌价。
	RiseRate  float64   // 涨跌幅。
}

// NewExGetKLine 构造 K 线查询协议实例；传入非空请求时覆盖默认参数。
func NewExGetKLine(req *ExGetKLineRequest) *ExGetKLine {
	obj := &ExGetKLine{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(ExGetKLineRequest),
		reply:      new(ExGetKLineReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXKLINE
	req.Count = req.Count + 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *ExGetKLine) applyRequest(req *ExGetKLineRequest) {
	if req.Times == 0 {
		req.Times = 1
	}
	obj.request = req
}

// BuildRequest 把当前请求参数编码为可发送的扩展协议字节流；序列化失败时返回非空错误。
func (obj *ExGetKLine) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXKLINE, payload.Bytes())
}

// ParseResponse 从服务端响应字节流解码 K 线列表并填充响应；日期编号解码失败或条目越界时返回非空错误。
func (obj *ExGetKLine) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 头部至少需要 20 字节。
	if len(data) < 20 {
		return fmt.Errorf("invalid ex kline response length: %d", len(data))
	}

	obj.reply.Category = data[0]
	obj.reply.Name = Utf8ToGbk(data[1:10])
	obj.reply.Period = binary.LittleEndian.Uint16(data[10:12])
	obj.reply.Times = binary.LittleEndian.Uint16(data[12:14])
	obj.reply.Start = binary.LittleEndian.Uint32(data[14:18])
	obj.reply.Count = binary.LittleEndian.Uint16(data[18:20])
	// K 线条目区起始偏移。
	pos := 20

	// 滚动保存上一根收盘价，作为本根昨收基准。
	var preCloseRaw float64 // 昨收盘价

	for i := uint16(0); i < obj.reply.Count; i++ {
		// 每根 K 线步长 32 字节，先校验剩余长度防止越界。
		if pos+32 > len(data) {
			return fmt.Errorf("invalid ex kline item %d", i)
		}
		dateNum := binary.LittleEndian.Uint32(data[pos : pos+4])
		// 日期编号按周期类型解码为时间戳，解码失败直接终止解析。
		ts, ok := decodeDateNum(obj.reply.Period, dateNum, false)
		if !ok {
			return fmt.Errorf("invalid ex kline datetime: %d", dateNum)
		}
		item := ExKLineItem{
			DateTime: ts,
			Open:     float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))),
			High:     float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))),
			Low:      float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+12 : pos+16]))),
			Close:    float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+16 : pos+20]))),
			Amount:   float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+20 : pos+24]))),
			Vol:      binary.LittleEndian.Uint32(data[pos+24 : pos+28]),
		}
		// 本根昨收/前收取上一根收盘价；随后把基准更新为本根收盘，供下一根使用。
		item.PreClose = float64(preCloseRaw)
		item.LastClose = float64(preCloseRaw)
		preCloseRaw = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+16 : pos+20])))
		item.RiseRate = item.GetRiseRate()
		item.RisePrice = item.GetRisePrice()
		pos += 32
		// 首根仅用于取得昨收基准，不进入最终 K 线列表。
		if i == 0 {
			continue
		}
		obj.reply.List = append(obj.reply.List, item)
	}
	obj.reply.Count = obj.reply.Count - 1
	return nil
}

// GetRisePrice 计算涨跌价，即收盘价相对昨收价的差值；昨收价为 0 时回退以开盘价为基准。
func (bar ExKLineItem) GetRisePrice() float64 {
	if bar.PreClose == 0 {
		// 昨收缺失时以开盘价为基准，差值相对更接近真实波动。
		return bar.Close - bar.Open
	}
	return bar.Close - bar.PreClose
}

// GetRiseRate 计算涨跌幅，即收盘价相对基准价的百分比；昨收价为 0 时回退以开盘价为基准。
func (bar ExKLineItem) GetRiseRate() float64 {
	if bar.PreClose == 0 {
		return float64(bar.Close-bar.Open) / float64(bar.Open) * 100
	}
	return float64(bar.Close-bar.PreClose) / float64(bar.PreClose) * 100
}

// Response 返回解码完成的 K 线响应对象。
func (obj *ExGetKLine) Response() *ExGetKLineReply {
	return obj.reply
}

// ExGetHistoryTransaction 表示扩展市场历史逐笔成交明细查询协议。
// 请求指定日期、市场类别、代码与笔数；响应包含证券名称、日期、起算基准价与逐笔成交记录。
// 实例必须通过 NewExGetHistoryTransaction 构造，零值不可用。
type ExGetHistoryTransaction struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *ExGetHistoryTransactionRequest
	reply      *ExGetHistoryTransactionReply
}

// ExGetHistoryTransactionRequest 保存历史逐笔成交查询的日期、市场类别、代码与笔数参数。
type ExGetHistoryTransactionRequest struct {
	Date     uint32   // 待查询日期编号。
	Category uint8    // 市场类别。
	Code     [43]byte // 证券代码。
	// Start    uint32
	Count uint16 // 请求成交笔数。
}

// ExGetHistoryTransactionReply 保存解码完成的历史逐笔成交响应结果。
type ExGetHistoryTransactionReply struct {
	Category   uint8                      // 市场类别。
	Name       string                     // 证券名称。
	Date       string                     // 交易日期。
	StartPrice float64                    // 当日起算基准价，语义待确认。
	Count      uint16                     // 返回成交笔数。
	List       []ExHistoryTransactionItem // 逐笔成交列表。
}

// ExHistoryTransactionItem 表示一笔历史逐笔成交，包含时间、成交价、成交量与买卖方向。
type ExHistoryTransactionItem struct {
	Time   string // 成交时间，格式为 `HH:MM`。
	Price  uint32 // 成交价（原始整数编码）。
	Vol    uint32 // 成交量。
	Action string // 买卖方向：买入、卖出或中性。
}

// NewExGetHistoryTransaction 构造历史逐笔成交查询协议实例；未指定笔数时默认请求 120 笔。
func NewExGetHistoryTransaction(req *ExGetHistoryTransactionRequest) *ExGetHistoryTransaction {
	obj := &ExGetHistoryTransaction{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(ExGetHistoryTransactionRequest),
		reply:      new(ExGetHistoryTransactionReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXHISTORYTRANSACTION
	// obj.request.Count = 0x78
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *ExGetHistoryTransaction) applyRequest(req *ExGetHistoryTransactionRequest) {
	if req.Count == 0 {
		req.Count = 0x78
	}
	obj.request = req
}

// BuildRequest 把当前请求参数编码为可发送的扩展协议字节流；序列化失败时返回非空错误。
func (obj *ExGetHistoryTransaction) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXHISTORYTRANSACTION, payload.Bytes())
}

// ParseResponse 从服务端响应字节流解码历史逐笔成交并填充响应；响应长度不足或成交记录越界时返回非空错误。
func (obj *ExGetHistoryTransaction) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 头部至少需要 58 字节。
	if len(data) < 58 {
		return fmt.Errorf("invalid ex history transaction response length: %d", len(data))
	}

	obj.reply.Category = data[0]
	obj.reply.Name = Utf8ToGbk(data[1:40])
	// 日期编号为 `YYYYMMDD` 整数，拆分后格式化为 `YYYY-MM-DD`。
	dateRaw := binary.LittleEndian.Uint32(data[40:44])
	obj.reply.Date = fmt.Sprintf("%04d-%02d-%02d", dateRaw/10000, (dateRaw%10000)/100, dateRaw%100)
	obj.reply.StartPrice = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[44:48])))
	obj.reply.Count = binary.LittleEndian.Uint16(data[56:58])
	// 成交条目区起始偏移。
	pos := 58

	for i := uint16(0); i < obj.reply.Count; i++ {
		// 每笔成交步长 16 字节，先校验剩余长度防止越界。
		if pos+16 > len(data) {
			return fmt.Errorf("invalid ex history transaction item %d", i)
		}
		minutes := binary.LittleEndian.Uint16(data[pos : pos+2])
		actionCode := binary.LittleEndian.Uint16(data[pos+14 : pos+16])
		// fmt.Println(data[pos+10:pos+16], actionCode)
		item := ExHistoryTransactionItem{
			Time:   fmt.Sprintf("%02d:%02d", (minutes/60)%24, minutes%60),
			Price:  binary.LittleEndian.Uint32(data[pos+2 : pos+6]),
			Vol:    binary.LittleEndian.Uint32(data[pos+6 : pos+10]),
			Action: exAction(actionCode),
		}
		obj.reply.List = append(obj.reply.List, item)
		pos += 16
	}
	return nil
}

// Response 返回解码完成的历史逐笔成交响应对象。
func (obj *ExGetHistoryTransaction) Response() *ExGetHistoryTransactionReply {
	return obj.reply
}

// exAction 将方向编码映射为买卖方向文本：0 为买入、1 为卖出、2 为中性，其余编码返回空字符串。
func exAction(code uint16) string {
	switch code {
	case 0:
		return "BUY"
	case 1:
		return "SELL"
	case 2:
		return "NEUTRAL"
	default:
		return ""
	}
}
