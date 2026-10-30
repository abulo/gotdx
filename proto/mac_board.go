package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// MACBoardListRequest 保存板块总数查询与板块列表查询共用的分页、板块类型与排序参数。
//
// 分页与排序相关字段为 0 时由构造函数回填默认值。
type MACBoardListRequest struct {
	PageSize  uint16
	BoardType uint16
	SortType  uint8
	SortOrder uint8
	Start     uint16
	One       uint16
	Reserved  [8]byte
}

// MACBoardCount 查询 MAC 板块总数，返回结果供上层计算分页。
//
// 使用时先由 NewMACBoardCount 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACBoardCount struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACBoardListRequest
	reply      *MACBoardCountReply
}

// MACBoardCountReply 保存板块总数查询返回的两组计数。
type MACBoardCountReply struct {
	CountAll uint16
	Total    uint16
}

// NewMACBoardCount 创建板块总数查询对象；请求参数为空时使用内置默认参数。
func NewMACBoardCount(req *MACBoardListRequest) *MACBoardCount {
	obj := &MACBoardCount{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACBoardListRequest),
		reply:      new(MACBoardCountReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXBOARDLIST
	obj.request.PageSize = 150
	obj.request.SortOrder = 1
	obj.request.One = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACBoardCount) applyRequest(req *MACBoardListRequest) {
	if req.PageSize == 0 {
		req.PageSize = 150
	}
	if req.SortOrder == 0 {
		req.SortOrder = 1
	}
	if req.One == 0 {
		req.One = 1
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACBoardCount) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXBOARDLIST, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足时返回错误。
func (obj *MACBoardCount) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 4 {
		return fmt.Errorf("invalid mac board count response length: %d", len(data))
	}
	// 两组计数均按小端序依次读取。
	obj.reply.CountAll = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Total = binary.LittleEndian.Uint16(data[2:4])
	return nil
}

// Response 返回解码完成的总数响应；未调用 ParseResponse 前内容为零值。
func (obj *MACBoardCount) Response() *MACBoardCountReply {
	return obj.reply
}

// MACBoardList 分页查询 MAC 板块列表，同时返回板块自身行情与代表标的行情。
//
// 使用时先由 NewMACBoardList 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACBoardList struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACBoardListRequest
	reply      *MACBoardListReply
}

// MACBoardListReply 保存板块列表查询返回的计数与逐条板块条目。
type MACBoardListReply struct {
	CountAll uint16
	Total    uint16
	Count    uint16
	List     []MACBoardListItem
}

// MACBoardListItem 表示一个板块条目；无前缀字段为板块自身行情，
// 带代表标的前缀的字段为该板块代表标的的行情快照。
type MACBoardListItem struct {
	Market          uint16
	Code            string
	Name            string
	Price           float64
	RiseSpeed       float64
	PreClose        float64
	SymbolMarket    uint16
	SymbolCode      string
	SymbolName      string
	SymbolPrice     float64
	SymbolRiseSpeed float64
	SymbolPreClose  float64
}

// NewMACBoardList 创建板块列表查询对象；请求参数为空时使用内置默认参数。
func NewMACBoardList(req *MACBoardListRequest) *MACBoardList {
	obj := &MACBoardList{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACBoardListRequest),
		reply:      new(MACBoardListReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXBOARDLIST
	obj.request.PageSize = 150
	obj.request.SortOrder = 1
	obj.request.One = 1
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACBoardList) applyRequest(req *MACBoardListRequest) {
	if req.PageSize == 0 {
		req.PageSize = 150
	}
	if req.SortOrder == 0 {
		req.SortOrder = 1
	}
	if req.One == 0 {
		req.One = 1
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACBoardList) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXBOARDLIST, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足或记录区不完整时返回错误。
func (obj *MACBoardList) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 4 {
		return fmt.Errorf("invalid mac board list response length: %d", len(data))
	}

	// 计数头两组计数均按小端序读取。
	obj.reply.CountAll = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Total = binary.LittleEndian.Uint16(data[2:4])
	// 计数通常按板块与标的成对统计，条目数先除以 2；折算为 0 而原始计数非 0 时回退，
	// 用于兼容服务端返回单条布局的情形。
	obj.reply.Count = obj.reply.CountAll / 2
	if obj.reply.Count == 0 && obj.reply.CountAll > 0 {
		obj.reply.Count = obj.reply.CountAll
	}

	// 记录区自偏移 4 开始，每条定长 160 字节。
	pos := 4
	for i := uint16(0); i < obj.reply.Count; i++ {
		// 剩余字节不足一条记录时中止并报错，防止越界读取。
		if pos+160 > len(data) {
			return fmt.Errorf("invalid mac board list item %d", i)
		}
		// 记录分两段：前半为板块自身行情，后半为代表标的行情；
		// 价格、涨速、昨收均为单精度浮点。
		item := MACBoardListItem{
			Market:          binary.LittleEndian.Uint16(data[pos : pos+2]),
			Code:            Utf8ToGbk(data[pos+2 : pos+8]),
			Name:            Utf8ToGbk(data[pos+24 : pos+68]),
			Price:           float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+68 : pos+72]))),
			RiseSpeed:       float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+72 : pos+76]))),
			PreClose:        float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+76 : pos+80]))),
			SymbolMarket:    binary.LittleEndian.Uint16(data[pos+80 : pos+82]),
			SymbolCode:      Utf8ToGbk(data[pos+82 : pos+88]),
			SymbolName:      Utf8ToGbk(data[pos+104 : pos+148]),
			SymbolPrice:     float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+148 : pos+152]))),
			SymbolRiseSpeed: float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+152 : pos+156]))),
			SymbolPreClose:  float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+156 : pos+160]))),
		}
		obj.reply.List = append(obj.reply.List, item)
		pos += 160
	}

	return nil
}

// Response 返回解码完成的板块列表响应；未调用 ParseResponse 前内容为零值。
func (obj *MACBoardList) Response() *MACBoardListReply {
	return obj.reply
}
