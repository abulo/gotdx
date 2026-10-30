package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// ExGetTickChart 表示扩展市场个股当日分时走势图查询协议，串联请求构造、报文编码、响应解码与结果获取。
//
// 该实例必须通过 NewExGetTickChart 构造，零值不可用；实例字段应在构造阶段确定。
type ExGetTickChart struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *ExGetTickChartRequest
	reply      *ExGetTickChartReply
}

// ExGetTickChartRequest 保存当日分时图查询的市场类别与证券代码参数。
type ExGetTickChartRequest struct {
	Category uint8    // 市场类别。
	Code     [23]byte // 证券代码。
	Reserved [8]byte  // 保留字段。
}

// ExGetTickChartReply 保存解码完成的当日分时图响应结果。
type ExGetTickChartReply struct {
	Category uint8             // 市场类别。
	Code     string            // 证券代码。
	Count    uint16            // 返回分钟点数。
	List     []ExTickChartData // 分时数据列表。
}

// ExTickChartData 表示分时图上的一个分钟点，包含时间、现价、均价与成交量。
type ExTickChartData struct {
	Time  string  // 分钟时间，格式为 `HH:MM`。
	Price float64 // 该分钟现价。
	Avg   float64 // 该分钟均价。
	Vol   int     // 该分钟成交量。
}

// NewExGetTickChart 构造当日分时图查询协议实例；传入非空请求时覆盖默认参数，
// 否则使用内置默认报文头与消息号。
func NewExGetTickChart(req *ExGetTickChartRequest) *ExGetTickChart {
	obj := &ExGetTickChart{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(ExGetTickChartRequest),
		reply:      new(ExGetTickChartReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXTICKCHART
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *ExGetTickChart) applyRequest(req *ExGetTickChartRequest) {
	obj.request = req
}

// BuildRequest 把当前请求参数编码为可发送的扩展协议字节流；序列化失败时返回非空错误。
func (obj *ExGetTickChart) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXTICKCHART, payload.Bytes())
}

// ParseResponse 从服务端响应字节流解码当日分时数据并填充响应；响应长度不足或分钟点越界时返回非空错误。
func (obj *ExGetTickChart) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 头部至少需要 34 字节。
	if len(data) < 34 {
		return fmt.Errorf("invalid ex tick chart response length: %d", len(data))
	}

	obj.reply.Category = data[0]
	obj.reply.Code = Utf8ToGbk(data[1:32])
	obj.reply.Count = binary.LittleEndian.Uint16(data[32:34])
	// 分钟点区起始偏移。
	pos := 34

	for i := uint16(0); i < obj.reply.Count; i++ {
		// 每个分钟点步长 18 字节，先校验剩余长度防止越界。
		if pos+18 > len(data) {
			return fmt.Errorf("invalid ex tick chart item %d", i)
		}
		minutes := binary.LittleEndian.Uint16(data[pos : pos+2])
		// 分钟数换算为 `HH:MM`；对 24 取模以兼容跨日溢出。
		obj.reply.List = append(obj.reply.List, ExTickChartData{
			Time:  fmt.Sprintf("%02d:%02d", (minutes/60)%24, minutes%60),
			Price: float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+2 : pos+6]))),
			Avg:   float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+6 : pos+10]))),
			Vol:   int(binary.LittleEndian.Uint32(data[pos+10 : pos+14])),
		})
		// 按定长步长推进到下一分钟点。
		pos += 18
	}
	return nil
}

// Response 返回解码完成的当日分时图响应对象。
func (obj *ExGetTickChart) Response() *ExGetTickChartReply {
	return obj.reply
}

// ExGetHistoryTickChart 表示扩展市场个股指定历史日期分时走势图查询协议。
//
// 响应头部比当日分时多携带证券名称、交易日期与当日均价，分钟点条目结构与当日分时一致。
type ExGetHistoryTickChart struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *ExGetHistoryTickChartRequest
	reply      *ExGetHistoryTickChartReply
}

// ExGetHistoryTickChartRequest 保存历史分时图查询的日期、市场类别与证券代码参数。
type ExGetHistoryTickChartRequest struct {
	Date     uint32   // 待查询日期编号。
	Category uint8    // 市场类别。
	Code     [23]byte // 证券代码。
	Reserved [6]byte  // 保留字段。
	Unknown  uint16   // 未知字段，语义待确认。
}

// ExGetHistoryTickChartReply 保存解码完成的历史分时图响应结果。
type ExGetHistoryTickChartReply struct {
	Category uint8             // 市场类别。
	Name     string            // 证券名称。
	Date     string            // 交易日期。
	AvgPrice float64           // 当日均价。
	Count    uint16            // 返回分钟点数。
	List     []ExTickChartData // 分时数据列表。
}

// NewExGetHistoryTickChart 构造历史分时图查询协议实例；传入非空请求时覆盖默认参数。
func NewExGetHistoryTickChart(req *ExGetHistoryTickChartRequest) *ExGetHistoryTickChart {
	obj := &ExGetHistoryTickChart{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(ExGetHistoryTickChartRequest),
		reply:      new(ExGetHistoryTickChartReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXHISTORYTICKCHART
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *ExGetHistoryTickChart) applyRequest(req *ExGetHistoryTickChartRequest) {
	obj.request = req
}

// BuildRequest 把当前请求参数编码为可发送的扩展协议字节流；序列化失败时返回非空错误。
func (obj *ExGetHistoryTickChart) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXHISTORYTICKCHART, payload.Bytes())
}

// ParseResponse 从服务端响应字节流解码历史分时数据并填充响应；响应长度不足或分钟点越界时返回非空错误。
func (obj *ExGetHistoryTickChart) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 头部至少需要 42 字节。
	if len(data) < 42 {
		return fmt.Errorf("invalid ex history tick chart response length: %d", len(data))
	}

	obj.reply.Category = data[0]
	obj.reply.Name = Utf8ToGbk(data[1:24])
	// 日期编号为 `YYYYMMDD` 整数，拆分后格式化为 `YYYY-MM-DD`。
	dateRaw := binary.LittleEndian.Uint32(data[24:28])
	obj.reply.Date = fmt.Sprintf("%04d-%02d-%02d", dateRaw/10000, (dateRaw%10000)/100, dateRaw%100)
	obj.reply.AvgPrice = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[28:32])))
	obj.reply.Count = binary.LittleEndian.Uint16(data[40:42])
	// 分钟点区起始偏移。
	pos := 42

	for i := uint16(0); i < obj.reply.Count; i++ {
		// 每个分钟点步长 18 字节，先校验剩余长度防止越界。
		if pos+18 > len(data) {
			return fmt.Errorf("invalid ex history tick chart item %d", i)
		}
		minutes := binary.LittleEndian.Uint16(data[pos : pos+2])
		obj.reply.List = append(obj.reply.List, ExTickChartData{
			Time:  fmt.Sprintf("%02d:%02d", (minutes/60)%24, minutes%60),
			Price: float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+2 : pos+6]))),
			Avg:   float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+6 : pos+10]))),
			Vol:   int(binary.LittleEndian.Uint32(data[pos+10 : pos+14])),
		})
		pos += 18
	}
	return nil
}

// Response 返回解码完成的历史分时图响应对象。
func (obj *ExGetHistoryTickChart) Response() *ExGetHistoryTickChartReply {
	return obj.reply
}

// ExGetChartSampling 表示扩展市场图表采样价格序列查询协议，用于走势图画线采样。
type ExGetChartSampling struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *ExGetChartSamplingRequest
	reply      *ExGetChartSamplingReply
}

// ExGetChartSamplingRequest 保存图表采样查询的市场类别、代码与采样点数参数。
type ExGetChartSamplingRequest struct {
	Category uint16   // 市场类别。
	Code     [22]byte // 证券代码。
	One      uint16   // 协议保留开关，语义待确认。
	Count    uint16   // 请求采样点数。
	Reserved [9]byte  // 保留字段。
}

// ExGetChartSamplingReply 保存解码完成的采样价格序列响应结果。
type ExGetChartSamplingReply struct {
	Category uint16    // 市场类别。
	Code     string    // 证券代码。
	Count    uint16    // 采样点数。
	Prices   []float64 // 采样价格序列。
}

// NewExGetChartSampling 构造图表采样查询协议实例；未指定开关与点数时默认开启采样并请求 20 个点。
func NewExGetChartSampling(req *ExGetChartSamplingRequest) *ExGetChartSampling {
	obj := &ExGetChartSampling{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(ExGetChartSamplingRequest),
		reply:      new(ExGetChartSamplingReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXCHARTSAMPLING
	obj.request.One = 1
	obj.request.Count = 20
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *ExGetChartSampling) applyRequest(req *ExGetChartSamplingRequest) {
	if req.One == 0 {
		req.One = 1
	}
	if req.Count == 0 {
		req.Count = 20
	}
	obj.request = req
}

// BuildRequest 把当前请求参数编码为可发送的扩展协议字节流；序列化失败时返回非空错误。
func (obj *ExGetChartSampling) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXCHARTSAMPLING, payload.Bytes())
}

// ParseResponse 从服务端响应字节流解码采样价格序列并填充响应；响应长度不足或采样点越界时返回非空错误。
func (obj *ExGetChartSampling) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 头部至少需要 42 字节。
	if len(data) < 42 {
		return fmt.Errorf("invalid ex chart sampling response length: %d", len(data))
	}

	obj.reply.Category = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Code = Utf8ToGbk(data[2:24])
	obj.reply.Count = binary.LittleEndian.Uint16(data[40:42])
	// 采样点区起始偏移。
	pos := 42

	for i := uint16(0); i < obj.reply.Count; i++ {
		// 每个采样点 4 字节，先校验剩余长度防止越界。
		if pos+4 > len(data) {
			return fmt.Errorf("invalid ex chart sampling item %d", i)
		}
		obj.reply.Prices = append(obj.reply.Prices, float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos:pos+4]))))
		pos += 4
	}
	return nil
}

// Response 返回解码完成的图表采样响应对象。
func (obj *ExGetChartSampling) Response() *ExGetChartSamplingReply {
	return obj.reply
}
