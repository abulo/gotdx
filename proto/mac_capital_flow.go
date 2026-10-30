package proto

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// MACCapitalFlow 查询单只标的的当日与五日资金流向。
//
// 使用时先由 NewMACCapitalFlow 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACCapitalFlow struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACCapitalFlowRequest
	reply      *MACCapitalFlowReply
}

// MACCapitalFlowRequest 保存资金流向查询的市场、标的代码与查询标识参数。
type MACCapitalFlowRequest struct {
	Market   uint16
	Symbol   [8]byte
	Reserved [16]byte
	Query    [21]byte
}

// MACCapitalFlowReply 保存资金流向响应的原始行数据与拆分后的当日、五日资金指标。
type MACCapitalFlowReply struct {
	Market           uint16
	QueryInfo        string
	Ext              string
	Today            []float64
	FiveDays         []float64
	TodayMainIn      float64
	TodayMainOut     float64
	TodayRetailIn    float64
	TodayRetailOut   float64
	TodayMainNetIn   float64
	TodayRetailNetIn float64
	FiveDayMainBuy   float64
	FiveDayMainSell  float64
	FiveDaySuperNet  float64
	FiveDayLargeNet  float64
	FiveDayMediumNet float64
	FiveDaySmallNet  float64
	FiveDayMainNetIn float64
}

// NewMACCapitalFlow 创建资金流向查询对象；请求参数为空时使用内置默认查询标识。
func NewMACCapitalFlow(req *MACCapitalFlowRequest) *MACCapitalFlow {
	obj := &MACCapitalFlow{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACCapitalFlowRequest),
		reply:      new(MACCapitalFlowReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACCAPITALFLOW
	obj.request.Query = makeMACCode21("Stock_ZJLX")
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACCapitalFlow) applyRequest(req *MACCapitalFlowRequest) {
	if req.Query == ([21]byte{}) {
		req.Query = makeMACCode21("Stock_ZJLX")
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACCapitalFlow) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildGenericRequest(0x02, 0, 0x01, KMSG_MACCAPITALFLOW, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足或负载不是合法 JSON 时返回错误。
func (obj *MACCapitalFlow) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 27 {
		return fmt.Errorf("invalid mac capital flow response length: %d", len(data))
	}

	// 定长文本头：市场（0:2）、查询文本（2:14）、扩展文本（19:27），均按 GBK 解码。
	obj.reply.Market = binary.LittleEndian.Uint16(data[:2])
	obj.reply.QueryInfo = Utf8ToGbk(data[2:14])
	obj.reply.Ext = Utf8ToGbk(data[19:27])

	// 27 字节之后整体为 JSON 二维数组；元素数值类型不固定，先解为通用切片再统一转换。
	var rows [][]interface{}
	if err := json.Unmarshal(data[27:], &rows); err != nil {
		return err
	}
	// 第一行为当日资金，第二行为五日资金；行数不足时对应分组保持为空。
	if len(rows) > 0 {
		obj.reply.Today = convertAnySliceToFloat64(rows[0])
	}
	if len(rows) > 1 {
		obj.reply.FiveDays = convertAnySliceToFloat64(rows[1])
	}
	// 当日列序为主力流入、主力流出、散户流入、散户流出；净额由流入减流出推算。
	if len(obj.reply.Today) >= 4 {
		obj.reply.TodayMainIn = obj.reply.Today[0]
		obj.reply.TodayMainOut = obj.reply.Today[1]
		obj.reply.TodayRetailIn = obj.reply.Today[2]
		obj.reply.TodayRetailOut = obj.reply.Today[3]
		obj.reply.TodayMainNetIn = obj.reply.TodayMainIn - obj.reply.TodayMainOut
		obj.reply.TodayRetailNetIn = obj.reply.TodayRetailIn - obj.reply.TodayRetailOut
	}
	// 五日列序：主力买入、主力卖出、超大单净额、大单净额、中单净额、小单净额。
	if len(obj.reply.FiveDays) >= 6 {
		obj.reply.FiveDayMainBuy = obj.reply.FiveDays[0]
		obj.reply.FiveDayMainSell = obj.reply.FiveDays[1]
		obj.reply.FiveDaySuperNet = obj.reply.FiveDays[2]
		obj.reply.FiveDayLargeNet = obj.reply.FiveDays[3]
		obj.reply.FiveDayMediumNet = obj.reply.FiveDays[4]
		obj.reply.FiveDaySmallNet = obj.reply.FiveDays[5]
		obj.reply.FiveDayMainNetIn = obj.reply.FiveDayMainBuy - obj.reply.FiveDayMainSell
	}
	return nil
}

// Response 返回解码完成的资金流向响应；未调用 ParseResponse 前内容为零值。
func (obj *MACCapitalFlow) Response() *MACCapitalFlowReply {
	return obj.reply
}

// convertAnySliceToFloat64 将 JSON 解出的任意类型数值切片逐项转换为浮点，
// 兼容 JSON 数字以普通浮点或原始数字文本等不同形态出现的情况。
func convertAnySliceToFloat64(values []interface{}) []float64 {
	out := make([]float64, 0, len(values))
	for _, value := range values {
		out = append(out, anyToFloat64(value))
	}
	return out
}
