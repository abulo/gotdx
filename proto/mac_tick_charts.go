package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// MACTickCharts 查询单只标的的多日分时序列与汇总行情。
//
// 使用时先由 NewMACTickCharts 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACTickCharts struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACTickChartsRequest
	reply      *MACTickChartsReply
}

// MACTickChartsRequest 保存多日分时查询的市场、代码、查询日期、交易日数与保留占位参数。
type MACTickChartsRequest struct {
	Market    uint16
	Code      [22]byte
	QueryDate uint32
	Days      uint16
	One       uint16
	Reserved  [6]byte
}

// MACTickChartsReply 保存多日分时响应的标的标识、交易日计数、逐日分时数据与汇总行情。
type MACTickChartsReply struct {
	Market       uint16
	Code         string
	Count        uint16
	SendLast     uint8
	PageSize     uint16
	Total        uint16
	Charts       []MACTickChartDay
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

// MACTickChartDay 表示一个交易日的分时数据，包含日期、当日昨收与该日分时点序列。
type MACTickChartDay struct {
	Date     string
	PreClose float64
	Ticks    []MACTickChartItem
}

// MACTickChartItem 表示一个分时点，包含时间标签、价格、均价、成交量与派生成交额；
// 其中一个保留槽位语义待确认，原样保存。
type MACTickChartItem struct {
	Time    string
	Price   float64
	Avg     float64
	Vol     uint16
	Amount  float64
	Unknown uint16
}

// NewMACTickCharts 创建多日分时查询对象；请求参数为空时使用内置默认交易日数。
func NewMACTickCharts(req *MACTickChartsRequest) *MACTickCharts {
	obj := &MACTickCharts{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACTickChartsRequest),
		reply:      new(MACTickChartsReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACTICKCHARTS
	obj.request.Days = 5
	obj.request.One = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACTickCharts) applyRequest(req *MACTickChartsRequest) {
	if req.Days == 0 {
		req.Days = 5
	}
	if req.One == 0 {
		req.One = 1
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACTickCharts) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACTICKCHARTS, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并按交易日分组填充响应；报文长度不足或记录区不完整时返回错误。
func (obj *MACTickCharts) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 71 {
		return fmt.Errorf("invalid mac tick charts response length: %d", len(data))
	}

	obj.reply.Market = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Code = Utf8ToGbk(data[2:24])

	// 头部预留最近五个交易日：日期数组位于 24 到 44，昨收数组位于 44 到 64，各五个四字节条目。
	var dates [5]uint32
	var preCloses [5]float32
	for i := range dates {
		dates[i] = binary.LittleEndian.Uint32(data[24+i*4 : 28+i*4])
		preCloses[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[44+i*4 : 48+i*4]))
	}

	// 偏移 64 到 71 依次为交易日数、末点标记、分页大小、总点数。
	obj.reply.Count = binary.LittleEndian.Uint16(data[64:66])
	obj.reply.SendLast = data[66]
	obj.reply.PageSize = binary.LittleEndian.Uint16(data[67:69])
	obj.reply.Total = binary.LittleEndian.Uint16(data[69:71])

	pos := 71
	// 临时保留分钟偏移，供分组时判断是否跨日。
	type tickWithMinute struct {
		minute uint16
		item   MACTickChartItem
	}

	ticks := make([]tickWithMinute, 0, obj.reply.Total)
	for tickIndex := uint16(0); tickIndex < obj.reply.Total; tickIndex++ {
		if pos+14 > len(data) {
			return fmt.Errorf("invalid mac tick charts item %d", tickIndex)
		}
		// 分时点布局：分钟偏移（0:2）、价格（2:6）、均价（6:10）、成交量（10:12）、保留槽位（12:14）。
		minutes := binary.LittleEndian.Uint16(data[pos : pos+2])
		MACTickChartItemInfo := MACTickChartItem{
			// 分钟偏移换算为时间标签，再整体后移一分钟以对齐行情展示口径。
			Time:    time.Date(0, 1, 1, int(minutes/60)%24, int(minutes%60), 0, 0, time.Local).Add(time.Minute).Format("15:04:05"),
			Price:   float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+2 : pos+6]))),
			Avg:     float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+6 : pos+10]))),
			Vol:     binary.LittleEndian.Uint16(data[pos+10 : pos+12]),
			Unknown: binary.LittleEndian.Uint16(data[pos+12 : pos+14]),
		}
		// 成交额由成交量、均价与固定系数 100 推算，报文未直接给出。
		MACTickChartItemInfo.Amount = float64(MACTickChartItemInfo.Vol) * MACTickChartItemInfo.Avg * 100
		ticks = append(ticks, tickWithMinute{
			minute: minutes,
			item:   MACTickChartItemInfo,
		})
		pos += 14
	}

	if obj.reply.Count == 0 {
		obj.reply.Charts = nil
	} else {
		currentDay := 0
		day := newMACTickChartDay(dates, preCloses, currentDay)
		for i, tick := range ticks {
			day.Ticks = append(day.Ticks, tick.item)
			// 最后一个交易日不再继续切分。
			if currentDay >= int(obj.reply.Count)-1 {
				continue
			}
			// 下一点分钟回绕到更小值，说明进入新交易日，封存当前日并开启新日。
			if i+1 < len(ticks) && ticks[i+1].minute <= tick.minute {
				obj.reply.Charts = append(obj.reply.Charts, day)
				currentDay++
				day = newMACTickChartDay(dates, preCloses, currentDay)
			}
		}
		obj.reply.Charts = append(obj.reply.Charts, day)
		// 交易日数多于实际分组时，补齐空交易日占位，使逐日序列长度与请求交易日数一致。
		for len(obj.reply.Charts) < int(obj.reply.Count) {
			obj.reply.Charts = append(obj.reply.Charts, newMACTickChartDay(dates, preCloses, len(obj.reply.Charts)))
		}
	}

	// 汇总行情段不足 120 字节时直接返回，跳过尾部字段解析。
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

// Response 返回解码完成的多日分时响应；未调用 ParseResponse 前内容为零值。
func (obj *MACTickCharts) Response() *MACTickChartsReply {
	return obj.reply
}

// formatMACDate 将打包日期整数拆出年、月、日，格式化为短横线分隔的日期文本，取本地时区零点。
func formatMACDate(raw uint32) string {
	year := int(raw / 10000)
	month := int((raw % 10000) / 100)
	day := int(raw % 100)
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local).Format("2006-01-02")
}

// newMACTickChartDay 按交易日序号从头部日期与昨收数组取出对应值，构造空的日分时容器；
// 序号越界或日期为零时保留空日期与零昨收。
func newMACTickChartDay(dates [5]uint32, preCloses [5]float32, dayIndex int) MACTickChartDay {
	day := MACTickChartDay{}
	if dayIndex < len(dates) && dates[dayIndex] != 0 {
		day.Date = formatMACDate(dates[dayIndex])
		day.PreClose = float64(preCloses[dayIndex])
	}
	return day
}
