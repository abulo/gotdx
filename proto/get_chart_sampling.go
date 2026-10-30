package proto

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
)

// GetChartSampling 封装图表抽样价格曲线（服务端压缩后的分时走势）查询的请求与响应处理流程。
//
// 服务端按图表可视宽度返回一组等间距抽样价格，用于直接绘制走势曲线，区别于逐分钟的历史分时数据。
type GetChartSampling struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetChartSamplingRequest
	reply      *GetChartSamplingReply
}

// GetChartSamplingRequest 是图表抽样查询的请求参数。
type GetChartSamplingRequest struct {
	Market   uint16   // 市场代码。
	Code     [6]byte  // 证券代码。
	Reserved [28]byte // 协议保留字段。
}

// GetChartSamplingReply 是图表抽样查询的响应数据。
type GetChartSamplingReply struct {
	Market   uint16    // 市场代码。
	Code     string    // 证券代码。
	Count    uint16    // 抽样价格点数。
	PreClose float64   // 昨收价。
	Prices   []float64 // 抽样价格序列。
}

// NewGetChartSampling 创建并初始化图表抽样查询请求对象；传入参数为空时沿用内置保留段模板。
func NewGetChartSampling(req *GetChartSamplingRequest) *GetChartSampling {
	obj := new(GetChartSampling)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetChartSamplingRequest)
	obj.reply = new(GetChartSamplingReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_CHARTSAMPLING
	copy(obj.request.Reserved[:], []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x14, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00,
	})
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetChartSampling) applyRequest(req *GetChartSamplingRequest) {
	if req.Reserved == [28]byte{} {
		req.Reserved = obj.request.Reserved
	}
	obj.request = req
}

// BuildRequest 先将请求参数按小端序编码为载荷，再由通用构造函数追加请求头，
// 组装为完整协议请求字节流；编码失败时返回错误。
func (obj *GetChartSampling) BuildRequest() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildGenericRequest(0x0c, 0, 0x01, KMSG_CHARTSAMPLING, buf.Bytes())
}

// ParseResponse 解析服务端返回的抽样价格序列并填充响应对象；报文不足定长报文头长度时返回截断错误。
//
// 前 42 字节为定长报文头：0:2 市场代码，2:8 证券代码，34:36 价格点数，36:40 昨收价；
// 偏移 42 起为逐点排列的价格序列，每个价格占 4 字节。
func (obj *GetChartSampling) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header

	// 最小报文长度校验：不足 42 字节定长报文头视为截断。
	if len(data) < 42 {
		return io.ErrUnexpectedEOF
	}

	// 0:2 市场代码。
	obj.reply.Market = binary.LittleEndian.Uint16(data[:2])
	// 2:8 六字节定长代码，解码后去除尾部填充。
	obj.reply.Code = Utf8ToGbk(data[2:8])
	// 34:36 抽样价格点数。
	obj.reply.Count = binary.LittleEndian.Uint16(data[34:36])
	// 36:40 昨收价。
	obj.reply.PreClose = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[36:40])))

	// 价格序列从偏移 42 开始，每点 4 字节；剩余字节不足时即停止，避免尾部截断越界。
	pos := 42
	for i := uint16(0); i < obj.reply.Count && pos+4 <= len(data); i++ {
		obj.reply.Prices = append(obj.reply.Prices, float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos:pos+4]))))
		pos += 4
	}

	return nil
}

// Response 返回解析完成后的图表抽样响应数据。
func (obj *GetChartSampling) Response() *GetChartSamplingReply {
	return obj.reply
}
