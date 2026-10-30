package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// MACBoardMembers 查询指定板块的成分股列表，仅返回市场、代码与名称。
//
// 使用时先由 NewMACBoardMembers 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACBoardMembers struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACBoardMembersRequest
	reply      *MACBoardMembersReply
}

// MACBoardMembersRequest 保存成分股列表查询的板块代码、排序、起始位置与分页参数。
type MACBoardMembersRequest struct {
	BoardCode uint32
	Reserved1 [9]byte
	SortType  uint16
	Start     uint32
	PageSize  uint8
	Zero      uint8
	SortOrder uint16
	Extra     [20]byte
}

// MACBoardMembersReply 保存成分股列表查询返回的板块名称、计数与逐条成分股。
type MACBoardMembersReply struct {
	Name   string
	Count  uint16
	Total  uint32
	Stocks []MACBoardMemberItem
}

// MACBoardMemberItem 表示一只板块成分股的市场、代码与名称。
type MACBoardMemberItem struct {
	Name   string
	Market uint16
	Symbol string
}

// NewMACBoardMembers 创建成分股列表查询对象；请求参数为空时使用内置默认参数。
func NewMACBoardMembers(req *MACBoardMembersRequest) *MACBoardMembers {
	obj := &MACBoardMembers{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACBoardMembersRequest),
		reply:      new(MACBoardMembersReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACBOARDMEMBERS
	obj.request.SortType = 14
	obj.request.PageSize = 80
	obj.request.SortOrder = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACBoardMembers) applyRequest(req *MACBoardMembersRequest) {
	if req.PageSize == 0 {
		req.PageSize = 80
	}
	if req.SortType == 0 {
		req.SortType = 14
	}
	if req.SortOrder == 0 {
		req.SortOrder = 1
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACBoardMembers) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACBOARDMEMBERS, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足或记录区不完整时返回错误。
func (obj *MACBoardMembers) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 响应头不足 26 字节时直接返回错误，避免后续切片越界。
	if len(data) < 26 {
		return fmt.Errorf("invalid mac board members response length: %d", len(data))
	}

	// 头部布局：GBK 板块名称（16:20）、成分股总数（20:24）、本次条数（24:26），均按小端序读取。
	obj.reply.Name = Utf8ToGbk(data[16:20])
	obj.reply.Total = binary.LittleEndian.Uint32(data[20:24])
	obj.reply.Count = binary.LittleEndian.Uint16(data[24:26])

	// 记录区自偏移 26 开始，每条定长 68 字节。
	pos := 26
	for i := uint16(0); i < obj.reply.Count; i++ {
		// 剩余字节不足一条记录时中止并报错，防止越界读取。
		if pos+68 > len(data) {
			return fmt.Errorf("invalid mac board member item %d", i)
		}
		// 记录布局：市场（0:2）、GBK 代码（2:8）、GBK 名称（24:40），中间偏移为填充或保留区。
		item := MACBoardMemberItem{
			Market: binary.LittleEndian.Uint16(data[pos : pos+2]),
			Symbol: Utf8ToGbk(data[pos+2 : pos+8]),
			Name:   Utf8ToGbk(data[pos+24 : pos+40]),
		}
		obj.reply.Stocks = append(obj.reply.Stocks, item)
		pos += 68
	}

	return nil
}

// Response 返回解码完成的成分股列表响应；未调用 ParseResponse 前内容为零值。
func (obj *MACBoardMembers) Response() *MACBoardMembersReply {
	return obj.reply
}

// MACBoardMembersQuotes 查询指定板块成分股的固定字段实时报价快照。
//
// 使用时先由 NewMACBoardMembersQuotes 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACBoardMembersQuotes struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACBoardMembersQuotesRequest
	reply      *MACBoardMembersQuotesReply
}

// MACBoardMembersQuotesRequest 保存成分股报价查询的板块代码、排序、分页与字段位图参数。
type MACBoardMembersQuotesRequest struct {
	BoardCode uint32
	Reserved1 [9]byte
	SortType  uint16
	Start     uint32
	PageSize  uint8
	Zero      uint8
	SortOrder uint8
	Extra     [21]byte
}

// MACBoardMembersQuotesReply 保存成分股报价查询返回的板块名称、计数与逐条报价。
type MACBoardMembersQuotesReply struct {
	Name   string
	Count  uint16
	Total  uint32
	Stocks []MACBoardMemberQuoteItem
}

// MACBoardMemberQuoteItem 表示一只成分股的固定字段实时报价快照。
//
// 槽位顺序与服务端 128 字节指标区一一对应；部分占位字段与别名字段的原始语义待确认，
// 实现未据此推断额外协议含义。
type MACBoardMemberQuoteItem struct {
	Name               string
	Market             uint16
	Symbol             string
	PreClose           float64
	Open               float64
	High               float64
	Low                float64
	Close              float64
	Unknown6           float64
	Vol                uint32
	VolumeRatio        float64
	Amount             float64
	TotalShares        float64
	FloatShares        float64
	EPS                float64
	ROE                float64
	NetAssets          float64
	ActionPrice        float64
	Unknown13          float64
	UnknownActionPrice float64
	MarketCap          float64
	TotalMarketCapAB   float64
	PEDynamic          float64
	Zero16             float64
	LotSizeInfo        uint32
	Unknown23          float64
	Zero17             float64
	DividendYield      float64
	RiseSpeed          float64
	CurrentVol         uint16
	LastVolume         uint32
	Turnover           float64
	TurnoverRate       float64
	Unknown21          float64
	SomeBitmap         uint32
	Unknown22          float64
	DecimalPoint       uint32
	LimitUp            float64
	BuyPriceLimit      float64
	LimitDown          float64
	SellPriceLimit     float64
	Zero25             float64
	Unknown34          uint32
	Unknown26          float64
	LotSize            uint32
	LotSizeBoardSymbol string
	Unknown27          float64
	PreIPOV            float64
	RiseSpeed2         float64
	SpeedPct           float64
	Zero29             float64
	FlagKCB            uint32
	KCBFlag            uint32
	PEStatic           float64
	PETTM              float64
	Unknown31          float64
	UnknownClosePrice  float64
}

// NewMACBoardMembersQuotes 创建成分股报价查询对象；请求参数为空时使用内置默认参数与默认位图。
func NewMACBoardMembersQuotes(req *MACBoardMembersQuotesRequest) *MACBoardMembersQuotes {
	obj := &MACBoardMembersQuotes{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACBoardMembersQuotesRequest),
		reply:      new(MACBoardMembersQuotesReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACBOARDMEMBERS
	obj.request.SortType = 14
	obj.request.PageSize = 80
	obj.request.SortOrder = 1
	obj.request.Extra = [21]byte{
		0x00, 0xff, 0xfc, 0xe1, 0xcc, 0x3f, 0x08, 0x03, 0x01, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00,
	}
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACBoardMembersQuotes) applyRequest(req *MACBoardMembersQuotesRequest) {
	if req.PageSize == 0 {
		req.PageSize = 80
	}
	if req.SortType == 0 {
		req.SortType = 14
	}
	if req.SortOrder == 0 {
		req.SortOrder = 1
	}
	if req.Extra == ([21]byte{}) {
		req.Extra = obj.request.Extra
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；写入失败时返回错误。
func (obj *MACBoardMembersQuotes) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	// 结构体对齐与线上字节布局不一致，不能整体序列化，需按线上字段顺序逐字段手工写入。
	if err := binary.Write(payload, binary.LittleEndian, obj.request.BoardCode); err != nil {
		return nil, err
	}
	// 9 字节保留区原样透传。
	if _, err := payload.Write(obj.request.Reserved1[:]); err != nil {
		return nil, err
	}
	// 排序类型、起始偏移、分页大小、保留字节、排序方向依次按线上顺序写入。
	if err := binary.Write(payload, binary.LittleEndian, obj.request.SortType); err != nil {
		return nil, err
	}
	if err := binary.Write(payload, binary.LittleEndian, obj.request.Start); err != nil {
		return nil, err
	}
	if err := binary.Write(payload, binary.LittleEndian, obj.request.PageSize); err != nil {
		return nil, err
	}
	if err := binary.Write(payload, binary.LittleEndian, obj.request.Zero); err != nil {
		return nil, err
	}
	if err := binary.Write(payload, binary.LittleEndian, obj.request.SortOrder); err != nil {
		return nil, err
	}
	// 21 字节位图扩展段整体写入。
	if _, err := payload.Write(obj.request.Extra[:]); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACBOARDMEMBERS, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足或记录区不完整时返回错误。
func (obj *MACBoardMembersQuotes) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 响应头不足 26 字节时直接返回错误，避免后续切片越界。
	if len(data) < 26 {
		return fmt.Errorf("invalid mac board members quotes response length: %d", len(data))
	}

	// 头部布局：GBK 板块名称（16:20）、成分股总数（20:24）、本次条数（24:26）。
	obj.reply.Name = Utf8ToGbk(data[16:20])
	obj.reply.Total = binary.LittleEndian.Uint32(data[20:24])
	obj.reply.Count = binary.LittleEndian.Uint16(data[24:26])

	pos := 26
	for i := uint16(0); i < obj.reply.Count; i++ {
		// 剩余字节不足一条记录时中止并报错，防止越界读取。
		if pos+196 > len(data) {
			return fmt.Errorf("invalid mac board member quote item %d", i)
		}
		// 身份段：市场（0:2）、GBK 代码（2:8）、GBK 名称（24:48）。
		item := MACBoardMemberQuoteItem{
			Market: binary.LittleEndian.Uint16(data[pos : pos+2]),
			Symbol: Utf8ToGbk(data[pos+2 : pos+8]),
			Name:   Utf8ToGbk(data[pos+24 : pos+48]),
		}

		// 指标区自偏移 68 起共 128 字节，每槽 4 字节，按槽位索引分别解码为浮点或计数。
		metrics := data[pos+68 : pos+196]
		// 闭包：将指定槽位的 4 字节还原为单精度浮点后再转双精度。
		floatAt := func(index int) float64 {
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(metrics[index*4 : index*4+4])))
		}
		// 闭包：将指定槽位按小端序读取为四字节无符号整数，用于计数、位图与精度标志。
		uintAt := func(index int) uint32 {
			return binary.LittleEndian.Uint32(metrics[index*4 : index*4+4])
		}

		// 槽位 0 到 4 为昨收、开、高、低、收；槽位 5 为成交量。
		item.PreClose = floatAt(0)
		item.Open = floatAt(1)
		item.High = floatAt(2)
		item.Low = floatAt(3)
		item.Close = floatAt(4)
		item.Vol = uintAt(5)
		item.Unknown6 = float64(item.Vol)
		// 槽位 6 到 17 为量比、成交额、股本、净资产、市值、动态市盈率、股息率等基本面指标。
		item.VolumeRatio = floatAt(6)
		item.Amount = floatAt(7)
		item.TotalShares = floatAt(8)
		item.FloatShares = floatAt(9)
		item.EPS = floatAt(10)
		item.NetAssets = floatAt(11)
		item.ROE = item.NetAssets
		item.ActionPrice = floatAt(12)
		item.UnknownActionPrice = item.ActionPrice
		item.Unknown13 = item.ActionPrice
		item.TotalMarketCapAB = floatAt(13)
		item.MarketCap = item.TotalMarketCapAB
		item.PEDynamic = floatAt(14)
		item.LotSizeInfo = uintAt(15)
		item.Zero16 = float64(item.LotSizeInfo)
		item.Unknown23 = floatAt(16)
		item.Zero17 = item.Unknown23
		item.DividendYield = floatAt(17)
		// 槽位 27 为涨速；槽位 18 起为现量、换手、标签位图、价格精度、涨跌停价等盘口字段。
		// 该分组沿用服务端固定槽位顺序，个别槽位的精确语义仍待确认。
		item.SpeedPct = floatAt(27)
		item.RiseSpeed = item.SpeedPct
		item.LastVolume = uintAt(18)
		item.CurrentVol = uint16(item.LastVolume)
		item.Turnover = floatAt(19)
		item.TurnoverRate = item.Turnover
		item.SomeBitmap = uintAt(20)
		item.Unknown21 = float64(item.SomeBitmap)
		item.DecimalPoint = uintAt(21)
		item.Unknown22 = float64(item.DecimalPoint)
		item.BuyPriceLimit = floatAt(22)
		item.LimitUp = item.BuyPriceLimit
		item.SellPriceLimit = floatAt(23)
		item.LimitDown = item.SellPriceLimit
		item.Unknown34 = uintAt(24)
		item.Zero25 = float64(item.Unknown34)
		item.LotSize = uintAt(25)
		item.LotSizeBoardSymbol = macLotSizeBoardSymbol(item.LotSize)
		item.Unknown26 = float64(item.LotSize)
		item.PreIPOV = floatAt(26)
		item.Unknown27 = item.PreIPOV
		item.RiseSpeed2 = item.SpeedPct
		// 槽位 28 到 31 为板块标志、滚动市盈率、静态市盈率及末槽位，末槽位语义待确认。
		item.KCBFlag = uintAt(28)
		item.FlagKCB = item.KCBFlag
		item.Zero29 = float64(item.KCBFlag)
		item.PETTM = floatAt(29)
		item.PEStatic = floatAt(30)
		item.UnknownClosePrice = floatAt(31)
		item.Unknown31 = item.UnknownClosePrice

		obj.reply.Stocks = append(obj.reply.Stocks, item)
		pos += 196
	}

	return nil
}

// Response 返回解码完成的成分股报价响应；未调用 ParseResponse 前内容为零值。
func (obj *MACBoardMembersQuotes) Response() *MACBoardMembersQuotesReply {
	return obj.reply
}
