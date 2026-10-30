package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// MACSymbolInfo 查询单只标的的股票摘要行情，包含身份、日期时间、开高低收、量额与内外盘等字段。
//
// 使用时先由 NewMACSymbolInfo 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACSymbolInfo struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACSymbolInfoRequest
	reply      *MACSymbolInfoReply
}

// MACSymbolInfoRequest 保存股票摘要查询的市场与标的代码参数，其余字段为协议保留占位。
type MACSymbolInfoRequest struct {
	Market   uint16
	Code     [22]byte
	One      uint32
	Reserved [12]byte
}

// MACSymbolInfoReply 保存股票摘要响应的全部字段；其中三个保留槽位的原始语义待确认，
// 仅按线上类型原样解码保存，未据此推断额外含义。
type MACSymbolInfoReply struct {
	Market        uint16
	Code          string
	Name          string
	DateTime      time.Time
	Activity      uint32
	PreClose      float64
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Momentum      float64
	Vol           uint32
	Amount        float64
	InsideVolume  uint32
	OutsideVolume uint32
	Decimal       uint16
	UnknownA      uint32
	UnknownB      float64
	UnknownC      uint32
	VR            float64
	Turnover      float64
	Avg           float64
}

// NewMACSymbolInfo 创建股票摘要查询对象；请求参数为空时使用内置默认参数。
func NewMACSymbolInfo(req *MACSymbolInfoRequest) *MACSymbolInfo {
	obj := &MACSymbolInfo{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACSymbolInfoRequest),
		reply:      new(MACSymbolInfoReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACSYMBOLINFO
	obj.request.One = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACSymbolInfo) applyRequest(req *MACSymbolInfoRequest) {
	if req.One == 0 {
		req.One = 1
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACSymbolInfo) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACSYMBOLINFO, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足 194 字节时返回错误。
func (obj *MACSymbolInfo) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 响应为 194 字节定长布局，长度不足即判定为非法报文，避免后续切片越界。
	if len(data) < 194 {
		return fmt.Errorf("invalid mac symbol info response length: %d", len(data))
	}

	// 身份段：市场（8:10）、GBK 代码（10:32）、GBK 名称（32:76）；76 到 96 为保留间隙。
	obj.reply.Market = binary.LittleEndian.Uint16(data[8:10])
	obj.reply.Code = Utf8ToGbk(data[10:32])
	obj.reply.Name = Utf8ToGbk(data[32:76])
	// 日期（96:100）与时间（100:104）为打包整数，组合为本地时间；104 到 108 为活跃度字段。
	dateRaw := binary.LittleEndian.Uint32(data[96:100])
	timeRaw := binary.LittleEndian.Uint32(data[100:104])
	obj.reply.DateTime = formatMACQuoteDateTime(dateRaw, timeRaw)
	obj.reply.Activity = binary.LittleEndian.Uint32(data[104:108])
	// 偏移 108 到 132 依次为昨收、开、高、低、收、动量，均为单精度浮点。
	obj.reply.PreClose = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[108:112])))
	obj.reply.Open = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[112:116])))
	obj.reply.High = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[116:120])))
	obj.reply.Low = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[120:124])))
	obj.reply.Close = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[124:128])))
	obj.reply.Momentum = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[128:132])))
	// 132:148 为成交量、成交额、内盘量、外盘量。
	obj.reply.Vol = binary.LittleEndian.Uint32(data[132:136])
	obj.reply.Amount = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[136:140])))
	obj.reply.InsideVolume = binary.LittleEndian.Uint32(data[140:144])
	obj.reply.OutsideVolume = binary.LittleEndian.Uint32(data[144:148])
	obj.reply.Decimal = binary.LittleEndian.Uint16(data[148:150])
	// 150:158 两个保留槽位（语义待确认）；158:178 为未映射间隙。
	obj.reply.UnknownA = binary.LittleEndian.Uint32(data[150:154])
	obj.reply.UnknownB = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[154:158])))
	obj.reply.UnknownC = binary.LittleEndian.Uint32(data[178:182])
	// 尾部 182:194：量比、换手率、均价。
	obj.reply.VR = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[182:186])))
	obj.reply.Turnover = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[186:190])))
	obj.reply.Avg = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[190:194])))
	return nil
}

// Response 返回解码完成的股票摘要响应；未调用 ParseResponse 前内容为零值。
func (obj *MACSymbolInfo) Response() *MACSymbolInfoReply {
	return obj.reply
}
