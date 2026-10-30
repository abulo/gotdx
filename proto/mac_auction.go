package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// MACAuction 查询单只标的的集合竞价撮合快照，包含时间、价格、撮合数量与买卖方向。
//
// 使用时先由 NewMACAuction 构造对象，再依次调用 BuildRequest 生成请求报文、
// ParseResponse 解码服务端返回字节流，最后读取响应结果。该对象面向单次请求使用，
// 内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACAuction struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACAuctionRequest
	reply      *MACAuctionReply
}

// MACAuctionRequest 保存 MAC 集合竞价请求的市场、代码、起始位置与条数参数。
//
// Count 为 0 时由构造函数回填默认条数；`Reserved` 为协议保留字节，调用方通常无需设置。
type MACAuctionRequest struct {
	Market   uint16
	Code     [22]byte
	Start    uint32
	Count    uint32
	Reserved [10]byte
}

// MACAuctionReply 保存 MAC 集合竞价响应的标的标识与逐条竞价快照。
type MACAuctionReply struct {
	Market uint16
	Code   string
	Count  uint32
	List   []MACAuctionItem
}

// MACAuctionItem 表示一条集合竞价撮合记录，包含时间、价格、已撮合与未撮合数量及买卖方向。
type MACAuctionItem struct {
	Time      string
	Price     float64
	Matched   uint32
	Unmatched int32
	Flag      int8
}

// NewMACAuction 创建集合竞价查询对象；请求参数为空时使用内置默认参数，条数为 0 时回填默认条数。
func NewMACAuction(req *MACAuctionRequest) *MACAuction {
	obj := &MACAuction{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACAuctionRequest),
		reply:      new(MACAuctionReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACAUCTION
	obj.request.Count = 500
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACAuction) applyRequest(req *MACAuctionRequest) {
	if req.Count == 0 {
		req.Count = 500
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACAuction) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACAUCTION, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应对象。
//
// 报文长度不足头部要求、或记录区不完整时返回错误；解码成功后可读取响应结果。
func (obj *MACAuction) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 响应头不足 36 字节时直接返回错误，避免后续切片越界。
	if len(data) < 36 {
		return fmt.Errorf("invalid mac auction response length: %d", len(data))
	}

	// 响应头布局：市场（0:2）、GBK 代码（2:24）、条数（24:28），均按小端序读取。
	obj.reply.Market = binary.LittleEndian.Uint16(data[:2])
	obj.reply.Code = Utf8ToGbk(data[2:24])
	obj.reply.Count = binary.LittleEndian.Uint32(data[24:28])

	// 记录区自偏移 36 开始，每条定长 16 字节。
	pos := 36
	for i := uint32(0); i < obj.reply.Count; i++ {
		// 剩余字节不足一条记录时中止并报错，防止越界读取。
		if pos+16 > len(data) {
			return fmt.Errorf("invalid mac auction item %d", i)
		}
		// 记录布局：当日秒数（0:4）、价格（4:8）、已撮合量（8:12）、未撮合量（12:16）；
		// 价格为单精度浮点，未撮合量为有符号整数。
		seconds := binary.LittleEndian.Uint32(data[pos : pos+4])
		unmatched := int32(binary.LittleEndian.Uint32(data[pos+12 : pos+16]))
		matched := binary.LittleEndian.Uint32(data[pos+8 : pos+12])
		// 未撮合量的符号位表示买卖方向：负数记为卖方，随后取绝对值保存。
		flag := int8(1)
		if unmatched < 0 {
			flag = -1
		}
		if unmatched < 0 {
			unmatched = -unmatched
		}
		obj.reply.List = append(obj.reply.List, MACAuctionItem{
			// 当日秒数拆分为时、分、秒；价格由单精度浮点位模式还原。
			Time:      time.Date(0, 1, 1, int(seconds/3600)%24, int((seconds%3600)/60), int(seconds%60), 0, time.Local).Format("15:04:05"),
			Price:     float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))),
			Matched:   matched,
			Unmatched: unmatched,
			Flag:      flag,
		})
		pos += 16
	}
	return nil
}

// Response 返回解码完成的竞价响应；未调用 ParseResponse 前内容为零值。
func (obj *MACAuction) Response() *MACAuctionReply {
	return obj.reply
}
