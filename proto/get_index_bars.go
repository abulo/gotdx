package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/bensema/gotdx/types"
	"github.com/spf13/cast"
)

// GetIndexBars 封装指数 K 线（含上涨、下跌家数）查询的完整请求与响应处理流程。
type GetIndexBars struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetIndexBarsRequest
	reply      *GetIndexBarsReply
}

// GetIndexBarsRequest 是指数 K 线查询的请求参数。
type GetIndexBarsRequest struct {
	Market   uint16  // 市场代码。
	Code     [6]byte // 指数代码。
	Category uint16  // K 线周期类别。
	Times    uint16  // 周期倍数。
	Start    uint16  // 起始偏移。
	Count    uint16  // 请求条数。
	Adjust   uint16  // 复权方式。
	Reserved [8]byte // 保留字段。
}

// GetIndexBarsReply 是指数 K 线查询的响应数据。
type GetIndexBarsReply struct {
	Count uint16     // 返回条数。
	List  []IndexBar // 指数 K 线列表。
}

// IndexBar 表示一根指数 K 线，含开高低收、成交量额、时间及上涨/下跌家数。
type IndexBar struct {
	PreClose  float64   // 昨收价
	LastClose float64   // 前收价
	RisePrice float64   // 涨跌价
	RiseRate  float64   // 涨跌幅
	Open      float64   // 开盘点位。
	Close     float64   // 收盘点位。
	High      float64   // 最高点位。
	Low       float64   // 最低点位。
	Vol       float64   // 成交量。
	Amount    float64   // 成交额。
	Year      int       // 年。
	Month     int       // 月。
	Day       int       // 日。
	Hour      int       // 时。
	Minute    int       // 分。
	DateTime  time.Time // K 线时间。
	UpCount   uint16    // 上涨家数。
	DownCount uint16    // 下跌家数。
}

// NewGetIndexBars 创建并初始化指数 K 线查询请求对象；传入参数为空时使用默认参数。
func NewGetIndexBars(req *GetIndexBarsRequest) *GetIndexBars {

	obj := new(GetIndexBars)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetIndexBarsRequest)
	obj.reply = new(GetIndexBarsReply)
	// 多取一根历史 K 线用于推算首根昨收价，解析阶段再舍弃首条多余记录。
	req.Count = req.Count + 1

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_INDEXBARS
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetIndexBars) applyRequest(req *GetIndexBarsRequest) {
	if req.Times == 0 {
		req.Times = 1
	}
	obj.request = req
}

// BuildRequest 将请求头与请求参数按小端序编码为协议字节流；编码失败时返回错误。
func (obj *GetIndexBars) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 0x1c
	obj.reqHeader.PkgLen2 = 0x1c

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, obj.reqHeader)
	err = binary.Write(buf, binary.LittleEndian, obj.request)
	return buf.Bytes(), err
}

// ParseResponse 解析服务端返回的指数 K 线数据并填充响应对象；报文长度不足时返回错误。
//
// 单根 K 线布局：4 字节日期编码，开、收、高、低四个变长价格，4 字节成交量，
// 4 字节成交额，4 字节上涨与下跌家数（两两交换字节序）。价格按千分刻度换算，
// 分钟级周期成交量先除 100 再乘 100 做单位归一；首根仅用于推算昨收，不进入结果列表。
func (obj *GetIndexBars) ParseResponse(header *RespHeader, data []byte) error {
	// 最少需要 2 字节条数头，否则直接判定数据不足。
	if len(data) < 2 {
		return errors.New("数据长度不足")
	}
	obj.respHeader = header
	// 0:2 K 线条数（含为推算昨收多取的那一根）。
	obj.reply.Count = Uint16(data[:2])
	data = data[2:]
	newCategory := cast.ToInt(obj.request.Category)
	// 缓存上一根收盘价，作为当前根的昨收价。
	var last int
	for index := uint16(0); index < obj.reply.Count; index++ {
		// 前 4 字节按当前周期类别解码为 K 线时间。
		dateTime := GetTime([4]byte(data[:4]), obj.request.Category)
		bar := IndexBar{
			DateTime: dateTime,
			Year:     dateTime.Year(),
			Month:    int(dateTime.Month()),
			Day:      dateTime.Day(),
			Hour:     dateTime.Hour(),
			Minute:   dateTime.Minute(),
		}
		// 开、收、高、低均为变长编码价格；解码函数消费对应字节并返回剩余缓冲。
		var open int
		data, open = GetPrice(data[4:])
		var close int
		data, close = GetPrice(data)
		var high int
		data, high = GetPrice(data)
		var low int
		data, low = GetPrice(data)

		// 价格刻度为千分之一，昨收沿用上一根收盘价。
		bar.PreClose = float64(last) / 1000.0
		bar.LastClose = float64(last) / 1000.0
		bar.Open = float64(open) / 1000.0
		bar.Close = float64(close) / 1000.0
		bar.High = float64(high) / 1000.0
		bar.Low = float64(low) / 1000.0
		last = close

		// 4 字节原始成交量，先按协议口径还原。
		bar.Vol = getVolume(Uint32(data[:4]))
		data = data[4:]

		// 分钟类周期成交量单位为手，先除 100 归一；日线等周期不做此缩放。
		switch newCategory {
		case types.KLINE_TYPE_EXHQ_1MIN, types.KLINE_TYPE_1MIN, types.KLINE_TYPE_5MIN, types.KLINE_TYPE_15MIN, types.KLINE_TYPE_30MIN, types.KLINE_TYPE_1HOUR:
			bar.Vol /= 100
		}
		// 4 字节成交额。
		bar.Amount = getVolume(Uint32(data[:4]))
		data = data[4:]

		// 成交量恢复到原始口径（乘回 100）。
		bar.Vol *= 100
		// 上涨家数与下跌家数均由两个字节交换顺序后小端还原。
		bar.UpCount = cast.ToUint16(Int([]byte{data[1], data[0]}))
		bar.DownCount = cast.ToUint16(Int([]byte{data[3], data[2]}))
		data = data[4:]

		// 依据昨收计算本根涨跌价与涨跌幅。
		bar.RiseRate = bar.GetRiseRate()
		bar.RisePrice = bar.GetRisePrice()

		// 首根是为推算昨收多取的记录，舍弃不放入结果列表。
		if index == 0 {
			continue
		}
		obj.reply.List = append(obj.reply.List, bar)
	}
	// 修正当天下午 13:00 被错映射为 11:30 的 K 线时间。
	obj.reply.List = FixKlineTimeIndexBar(obj.reply.List)
	// 结果条数扣除开头被舍弃的那一根。
	obj.reply.Count = obj.reply.Count - 1

	return nil
}

// GetRisePrice 计算涨跌价（收盘点位相对昨收点位的差值）。
// 昨收为 0（首根无历史基准）时退化为收盘价减开盘价，避免出现负昨收导致的异常差值。
func (bar IndexBar) GetRisePrice() float64 {
	if bar.PreClose == 0 {
		// 首根无历史昨收基准，以开盘价代替，避免零昨收造成失真差值。
		return bar.Close - bar.Open
	}
	return bar.Close - bar.PreClose
}

// GetRiseRate 计算涨跌幅（百分比）。
// 昨收为 0 时以开盘价为基准计算；否则以昨收价为基准，结果乘以 100 得到百分数。
func (bar IndexBar) GetRiseRate() float64 {
	if bar.PreClose == 0 {
		return float64(bar.Close-bar.Open) / float64(bar.Open) * 100
	}
	return float64(bar.Close-bar.PreClose) / float64(bar.PreClose) * 100
}

// Response 返回解析完成后的指数 K 线响应数据。
func (obj *GetIndexBars) Response() *GetIndexBarsReply {
	return obj.reply
}

// FixKlineTimeIndexBar 修正当天下午首根 K 线的时间错位问题；非当天盘中数据原样返回。
//
// 服务端下午开盘首根分钟 K 线存在时间错位，本函数把落在 13:00 的记录改写为 11:30。
func FixKlineTimeIndexBar(ks []IndexBar) []IndexBar {
	if len(ks) == 0 {
		return ks
	}
	now := time.Now()
	// 只有当天下午 13~15 点之间才会出现该时间错位问题。
	node1 := time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, now.Location())
	node2 := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())
	// 最后一根不在下午交易窗口内，说明不是当天盘中数据，直接原样返回。
	if ks[len(ks)-1].DateTime.Unix() < node1.Unix() || ks[len(ks)-1].DateTime.Unix() > node2.Unix() {
		return ks
	}
	// 只取末尾最多 120 根进行检查，避免全量遍历。
	ls := ks
	if len(ls) >= 120 {
		ls = ls[len(ls)-120:]
	}
	for i, v := range ls {
		// 被错标为 13:00 的记录，回写为午间收盘 11:30。
		if v.DateTime.Unix() == node1.Unix() {
			ls[i].DateTime = time.Date(now.Year(), now.Month(), now.Day(), 11, 30, 0, 0, now.Location())
		}
	}
	return ks
}
