package proto

import (
	"bytes"
	"encoding/binary"
	"math"
)

// GetVolumeProfile 表示一次获取个股成交分布与盘口快照协议的完整流程对象。
//
// 成交分布返回各价位上的成交量分布明细，响应前段附带一份与盘口协议同构的快照。
// 零值不可直接使用，必须通过 NewGetVolumeProfile 构造；对象绑定单次查询流程，
// 不支持多协程并发复用。
type GetVolumeProfile struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetVolumeProfileRequest
	reply      *GetVolumeProfileReply
}

// GetVolumeProfileRequest 是 GetVolumeProfile 成交分布查询使用的请求参数。
type GetVolumeProfileRequest struct {
	Market uint16  // 市场代码。
	Code   [6]byte // 证券代码。
}

// GetVolumeProfileReply 是 GetVolumeProfile 成交分布查询解析完成后的响应数据，包含盘口快照与各价位成交分布档。
type GetVolumeProfileReply struct {
	Count       uint16              // 成交分布档位数。
	Market      uint8               // 市场代码。
	Code        string              // 证券代码。
	Active      uint16              // 活跃度字段，语义待确认。
	Close       float64             // 最新价，单位为元。
	Open        float64             // 今开价，单位为元。
	High        float64             // 最高价，单位为元。
	Low         float64             // 最低价，单位为元。
	PreClose    float64             // 昨收价，单位为元。
	ServerTime  string              // 服务端时间。
	NegPrice    float64             // 特殊价格字段，语义待确认。
	Vol         int                 // 总成交量。
	CurVol      int                 // 现量。
	Amount      float64             // 总成交额。
	InVol       int                 // 内盘量。
	OutVol      int                 // 外盘量。
	SAmount     int                 // 上涨家数或卖出相关统计字段，语义待确认。
	OpenAmount  int                 // 开盘金额。
	Turnover    float64             // 换手率，由高层接口尽力估算补齐。
	BidLevels   []Level             // 买盘三档。
	AskLevels   []Level             // 卖盘三档。
	Unknown     uint16              // 未确认字段，语义待确认。
	VolProfiles []VolumeProfileItem // 成交分布明细列表。
}

// VolumeProfileItem 表示成交分布中的一个价格档位，包含该档总量与主买、主卖量。
type VolumeProfileItem struct {
	Price float64 // 档位价格，单位为元。
	Vol   int     // 该档总成交量。
	Buy   int     // 主买量。
	Sell  int     // 主卖量。
}

// NewGetVolumeProfile 构造成交分布查询对象；`req` 为 `nil` 时使用空请求参数，返回的对象可直接调用 BuildRequest 与 ParseResponse。
func NewGetVolumeProfile(req *GetVolumeProfileRequest) *GetVolumeProfile {
	obj := new(GetVolumeProfile)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetVolumeProfileRequest)
	obj.reply = new(GetVolumeProfileReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_VOLUMEPROFILE
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetVolumeProfile) applyRequest(req *GetVolumeProfileRequest) {
	obj.request = req
}

// BuildRequest 按小端序将请求头与请求参数编码为完整协议报文；缓冲区写入失败时返回错误。
func (obj *GetVolumeProfile) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 0x0a
	obj.reqHeader.PkgLen2 = 0x0a

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, obj.reqHeader)
	err = binary.Write(buf, binary.LittleEndian, obj.request)
	return buf.Bytes(), err
}

// ParseResponse 解析服务端返回的盘口快照与成交分布档位报文并填充响应对象；解码失败时返回错误。
func (obj *GetVolumeProfile) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header

	// 快照头部为定长字段，条数、市场、代码、活跃度的偏移依次固定为 0、2、3、9。
	obj.reply.Count = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Market = data[2]
	obj.reply.Code = Utf8ToGbk(data[3:9])
	obj.reply.Active = binary.LittleEndian.Uint16(data[9:11])
	pos := 11

	// 基准价与昨收、今开、最高、最低差值；后续价位为基准价加差值。
	basePrice := getprice(data, &pos)
	preCloseDiff := getprice(data, &pos)
	openDiff := getprice(data, &pos)
	highDiff := getprice(data, &pos)
	lowDiff := getprice(data, &pos)
	serverTimeRaw := getprice(data, &pos)
	negPriceRaw := getprice(data, &pos)
	obj.reply.Vol = getprice(data, &pos)
	obj.reply.CurVol = getprice(data, &pos)
	// 成交额为 4 字节小端序单精度浮点数。
	obj.reply.Amount = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4])))
	pos += 4
	obj.reply.InVol = getprice(data, &pos)
	obj.reply.OutVol = getprice(data, &pos)
	obj.reply.SAmount = getprice(data, &pos)
	obj.reply.OpenAmount = getprice(data, &pos)

	// 三档买卖盘：买/卖价为基准价加档位差值，档位量独立读取。
	for i := 0; i < 3; i++ {
		bid := getprice(data, &pos) + basePrice
		ask := getprice(data, &pos) + basePrice
		bidVol := getprice(data, &pos)
		askVol := getprice(data, &pos)
		obj.reply.BidLevels = append(obj.reply.BidLevels, Level{Price: float64(bid) / 100.0, Vol: bidVol})
		obj.reply.AskLevels = append(obj.reply.AskLevels, Level{Price: float64(ask) / 100.0, Vol: askVol})
	}

	obj.reply.Unknown = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2

	// 分布明细：累计价位逐条叠加差值得到当前档位绝对价位。
	profilePrice := 0
	for i := uint16(0); i < obj.reply.Count; i++ {
		// 价位差值可能以 32 位补码变长编码返回，先做溢出归一化再累加。
		priceDelta := decodeVolumeProfilePriceDelta(getprice(data, &pos))
		vol := getprice(data, &pos)
		buy := getprice(data, &pos)
		sell := getprice(data, &pos)
		profilePrice += priceDelta
		obj.reply.VolProfiles = append(obj.reply.VolProfiles, VolumeProfileItem{
			Price: float64(profilePrice) / 100.0,
			Vol:   vol,
			Buy:   buy,
			Sell:  sell,
		})
	}

	// 快照价位统一以基准价加差值还原，除以 100 得到元。
	obj.reply.Close = float64(basePrice) / 100.0
	obj.reply.Open = float64(basePrice+openDiff) / 100.0
	obj.reply.High = float64(basePrice+highDiff) / 100.0
	obj.reply.Low = float64(basePrice+lowDiff) / 100.0
	obj.reply.PreClose = float64(basePrice+preCloseDiff) / 100.0
	obj.reply.ServerTime = formatServerTime(serverTimeRaw)
	obj.reply.NegPrice = float64(negPriceRaw) / 100.0

	return nil
}

// Response 返回已解析完成的成交分布响应数据；未调用 ParseResponse 时返回空响应对象。
func (obj *GetVolumeProfile) Response() *GetVolumeProfileReply {
	return obj.reply
}

// decodeVolumeProfilePriceDelta 归一化成交分布的价位差值。
//
// 变长编码按有符号整数解析，但分布档位的负差值可能以 32 位补码形式返回，高位被置 1
// 导致结果溢出为大正数；当数值达到或超过 2 的 31 次方时，先转回无符号 32 位整数再按
// 有符号 32 位整数解释，把溢出值还原为正确的负差值，其余数值原样返回。
func decodeVolumeProfilePriceDelta(value int) int {
	// 高位被置 1：按 32 位补码重新解释为有符号整数。
	if value >= 1<<31 {
		return int(int32(uint32(value)))
	}
	return value
}
