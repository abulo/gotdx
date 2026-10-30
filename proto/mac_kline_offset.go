package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// MACKLineOffset 查询 K 线总数与服务端本次实际返回条数，供上层换算分页位置与剩余数量。
//
// 使用时先由 NewMACKLineOffset 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACKLineOffset struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACKLineOffsetRequest
	reply      *MACKLineOffsetReply
}

// MACKLineOffsetRequest 保存偏移查询的起始偏移与期望条数参数。
type MACKLineOffsetRequest struct {
	Offset   uint32
	Count    uint32
	Reserved [5]byte
}

// MACKLineOffsetReply 保存偏移查询返回的 K 线总数与本次实际返回条数。
type MACKLineOffsetReply struct {
	Total    uint32
	Returned uint32
}

// NewMACKLineOffset 创建偏移查询对象；请求参数为空时使用内置默认条数。
func NewMACKLineOffset(req *MACKLineOffsetRequest) *MACKLineOffset {
	obj := &MACKLineOffset{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    &MACKLineOffsetRequest{Count: 128000},
		reply:      new(MACKLineOffsetReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACKLINEOFFSET
	// 默认一次请求 128000 条记录；调用方传入非零条数时覆盖该默认值。
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACKLineOffset) applyRequest(req *MACKLineOffsetRequest) {
	if req.Count == 0 {
		req.Count = 128000
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACKLineOffset) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACKLINEOFFSET, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足 8 字节时返回错误。
func (obj *MACKLineOffset) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 8 {
		return fmt.Errorf("invalid mac kline offset response length: %d", len(data))
	}
	// 该报文为混合端序：总数按大端序读取，本次返回条数按小端序读取，不能统一处理。
	obj.reply.Total = binary.BigEndian.Uint32(data[:4])
	obj.reply.Returned = binary.LittleEndian.Uint32(data[4:8])
	return nil
}

// Response 返回解码完成的偏移查询响应；未调用 ParseResponse 前内容为零值。
func (obj *MACKLineOffset) Response() *MACKLineOffsetReply {
	return obj.reply
}
