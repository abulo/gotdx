package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// MACSymbolBars 查询单只标的的 K 线序列与汇总行情。
//
// 使用时先由 NewMACSymbolBars 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACSymbolBars struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACSymbolBarsRequest
	reply      *MACSymbolBarsReply
}

// MACSymbolBarsRequest 保存 K 线查询的市场、代码、周期、起始位置、条数与复权标志参数。
type MACSymbolBarsRequest struct {
	Market   uint16
	Code     [22]byte
	Period   uint16
	Times    uint16
	Start    uint32
	Count    uint16
	Adjust   uint16
	Flag1    int8
	Flag2    int8
	Flag3    int8
	Flag4    int8
	Zero     uint16
	Reserved [4]byte
}

// MACSymbolBarsReply 保存 K 线响应的标的标识、K 线序列与汇总行情数据。
type MACSymbolBarsReply struct {
	Market       uint16
	Code         string
	Period       uint8
	Unknown      uint16 // 协议保留槽位，语义待确认
	Count        uint16
	Start        uint32
	List         []MACSymbolBar
	Name         string
	Decimal      uint8
	Category     uint16
	VolUnit      float64
	DateTime     time.Time
	PreClose     float64
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Momentum     float64
	Vol          uint32
	Amount       float64
	Turnover     float64
	Avg          float64
	Industry     uint32
	IndustryCode string
}

// MACSymbolBar 表示一根 K 线，包含开高低收、量额、流通股本、昨收基准与涨跌信息。
type MACSymbolBar struct {
	DateTime    time.Time
	RawYMD      uint32 // 原始交易日字段，未经时间修正
	RawSeconds  uint32 // 原始当日秒数字段，未经时间修正
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Amount      float64
	Vol         float64
	FloatShares float64
	Turnover    float64
	PreClose    float64 // 昨收价
	LastClose   float64 // 前收价
	RisePrice   float64 // 涨跌价
	RiseRate    float64 // 涨跌幅
}

// NewMACSymbolBars 创建 K 线查询对象；请求参数为空时使用内置默认参数。
func NewMACSymbolBars(req *MACSymbolBarsRequest) *MACSymbolBars {
	obj := &MACSymbolBars{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACSymbolBarsRequest),
		reply:      new(MACSymbolBarsReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACSYMBOLBARS
	obj.request.Times = 1
	obj.request.Flag1 = 1
	obj.request.Flag2 = 1
	obj.request.Flag4 = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACSymbolBars) applyRequest(req *MACSymbolBarsRequest) {
	if req.Times == 0 {
		req.Times = 1
	}
	if req.Flag1 == 0 {
		req.Flag1 = 1
	}
	if req.Flag2 == 0 {
		req.Flag2 = 1
	}
	if req.Flag4 == 0 {
		req.Flag4 = 1
	}
	// 请求条数加一：多取一条作为首根 K 线的昨收基准，解析阶段再舍弃该首条。
	req.Count = req.Count + 1
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACSymbolBars) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACSYMBOLBARS, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足或 K 线记录区不完整时返回错误。
func (obj *MACSymbolBars) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 33 {
		return fmt.Errorf("invalid mac symbol bars response length: %d", len(data))
	}

	// 头部：市场（0:2）、GBK 代码（2:14）、周期（24）、计数与起始偏移（27:33）。
	obj.reply.Market = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Code = Utf8ToGbk(data[2:14])
	obj.reply.Period = data[24]
	obj.reply.Unknown = binary.LittleEndian.Uint16(data[25:27])
	obj.reply.Count = binary.LittleEndian.Uint16(data[27:29])
	obj.reply.Start = binary.LittleEndian.Uint32(data[29:33])

	pos := 33
	var preCloseRaw float64 // 滚动保存上一根收盘价，作为当前根的昨收价
	for i := uint16(0); i < obj.reply.Count; i++ {
		if pos+36 > len(data) {
			return fmt.Errorf("invalid mac symbol bar item %d", i)
		}
		// K 线记录布局：打包日期（0:4）、当日秒数（4:8），其后八段单精度浮点依次为
		// 开、高、低、收、成交额、成交量、流通股本。
		ymd := binary.LittleEndian.Uint32(data[pos : pos+4])
		seconds := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		item := MACSymbolBar{
			DateTime:    combineMACDateTime(ymd, seconds),
			RawYMD:      ymd,
			RawSeconds:  seconds,
			Open:        float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))),
			High:        float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+12 : pos+16]))),
			Low:         float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+16 : pos+20]))),
			Close:       float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+20 : pos+24]))),
			Amount:      float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+24 : pos+28]))),
			Vol:         float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+28 : pos+32]))),
			FloatShares: float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+32 : pos+36]))),
		}
		// 昨收与前收取上一根收盘价，随后把基准滚动为当前收盘价，再计算涨跌价与涨跌幅。
		item.PreClose = preCloseRaw
		item.LastClose = preCloseRaw
		preCloseRaw = item.Close
		item.RiseRate = item.GetRiseRate()
		item.RisePrice = item.GetRisePrice()
		pos += 36
		// 首条仅用于初始化昨收基准，不进入结果列表。
		if i == 0 {
			continue
		}
		obj.reply.List = append(obj.reply.List, item)
	}
	// 结果条数比报文条数少一，即被舍弃的首条基准。
	obj.reply.Count = obj.reply.Count - 1

	// 汇总行情段不足 120 字节时直接返回，不再解析尾部字段。
	if pos+120 > len(data) {
		return nil
	}

	// 汇总段布局与行情快照尾部一致：名称、精度、类别、量纲、日期时间、开高低收、量额、换手、均价、行业。
	obj.reply.Name = Utf8ToGbk(data[pos : pos+44])
	obj.reply.Decimal = data[pos+44]
	obj.reply.Category = binary.LittleEndian.Uint16(data[pos+45 : pos+47])
	obj.reply.VolUnit = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+47 : pos+51])))
	dateRaw := binary.LittleEndian.Uint32(data[pos+56 : pos+60])
	timeRaw := binary.LittleEndian.Uint32(data[pos+60 : pos+64])
	obj.reply.DateTime = formatMACQuoteDateTime(dateRaw, timeRaw)
	obj.reply.PreClose = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+64 : pos+68])))
	obj.reply.Open = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+68 : pos+72])))
	obj.reply.High = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+72 : pos+76])))
	obj.reply.Low = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+76 : pos+80])))
	obj.reply.Close = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+80 : pos+84])))
	obj.reply.Momentum = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+84 : pos+88])))
	obj.reply.Vol = binary.LittleEndian.Uint32(data[pos+88 : pos+92])
	obj.reply.Amount = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+92 : pos+96])))
	obj.reply.Turnover = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+108 : pos+112])))
	obj.reply.Avg = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+112 : pos+116])))
	obj.reply.Industry = binary.LittleEndian.Uint32(data[pos+116 : pos+120])
	obj.reply.IndustryCode = macIndustryBoardSymbol(obj.reply.Industry)

	return nil
}

// GetRisePrice 返回收盘价相对昨收价的涨跌价；昨收价为 0 时改用开盘价作基准，避免差值失真。
func (bar MACSymbolBar) GetRisePrice() float64 {
	if bar.PreClose == 0 {
		// 昨收缺失时退用开盘价，避免差值被放大为接近收盘价本身。
		return bar.Close - bar.Open
	}
	return bar.Close - bar.PreClose
}

// GetRiseRate 返回以昨收价为基准的涨跌幅百分比；昨收价为 0 时改用开盘价作基准。
func (bar MACSymbolBar) GetRiseRate() float64 {
	if bar.PreClose == 0 {
		return (bar.Close - bar.Open) / (bar.Open) * 100
	}
	return (bar.Close - bar.PreClose) / (bar.PreClose) * 100
}

// Response 返回解码完成的 K 线响应；未调用 ParseResponse 前内容为零值。
func (obj *MACSymbolBars) Response() *MACSymbolBarsReply {
	return obj.reply
}

// combineMACDateTime 组合打包交易日与当日秒数为本地时间，不另行推断自然日期。
func combineMACDateTime(ymd uint32, seconds uint32) time.Time {
	year := int(ymd / 10000)
	month := int((ymd % 10000) / 100)
	day := int(ymd % 100)
	hours := int(seconds / 3600)
	minutes := int((seconds % 3600) / 60)

	return time.Date(year, time.Month(month), day, hours, minutes, 0, 0, time.Local)
}
