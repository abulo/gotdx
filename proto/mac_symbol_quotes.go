package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// MACSymbolQuotes 按位图批量查询多只标的的实时报价。
//
// 使用时先由 NewMACSymbolQuotes 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACSymbolQuotes struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACSymbolQuotesRequest
	reply      *MACSymbolQuotesReply
}

// MACSymbolQuoteStock 表示一个批量报价查询目标的市场与代码。
type MACSymbolQuoteStock struct {
	Market uint16
	Code   [22]byte
}

// MACSymbolQuotesRequest 保存批量报价查询的字段位图与待查询标的列表。
type MACSymbolQuotesRequest struct {
	FieldBitmap [20]byte
	Stocks      []MACSymbolQuoteStock
}

// MACSymbolQuoteItem 表示一只标的本次位图激活的全部动态字段报价结果，取值以字段名与别名为键存入映射。
type MACSymbolQuoteItem struct {
	Name   string
	Market uint16
	Symbol string
	Values map[string]any
}

// MACSymbolQuotesReply 保存批量报价响应的回显位图、激活字段定义、计数与逐条标的报价。
type MACSymbolQuotesReply struct {
	FieldBitmap  [20]byte
	ActiveFields []MACDynamicFieldDef
	Count        uint16
	Total        uint32
	Stocks       []MACSymbolQuoteItem
}

// NewMACSymbolQuotes 创建批量报价查询对象；请求参数为空时使用内置默认位图。
func NewMACSymbolQuotes(req *MACSymbolQuotesRequest) *MACSymbolQuotes {
	obj := &MACSymbolQuotes{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACSymbolQuotesRequest),
		reply:      new(MACSymbolQuotesReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACSYMBOLQUOTES
	obj.request.FieldBitmap = defaultMACBoardMembersQuotesFieldBitmap()
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACSymbolQuotes) applyRequest(req *MACSymbolQuotesRequest) {
	if req.FieldBitmap == ([20]byte{}) {
		req.FieldBitmap = defaultMACBoardMembersQuotesFieldBitmap()
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；写入失败时返回错误。
func (obj *MACSymbolQuotes) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	// 先写入字段位图，声明本次需要返回的指标位。
	if _, err := payload.Write(obj.request.FieldBitmap[:]); err != nil {
		return nil, err
	}
	// 标的个数按小端序写入。
	if err := binary.Write(payload, binary.LittleEndian, uint16(len(obj.request.Stocks))); err != nil {
		return nil, err
	}
	// 每个目标依次写入市场与定长 22 字节代码。
	for _, stock := range obj.request.Stocks {
		if err := binary.Write(payload, binary.LittleEndian, stock.Market); err != nil {
			return nil, err
		}
		if err := binary.Write(payload, binary.LittleEndian, stock.Code); err != nil {
			return nil, err
		}
	}
	return buildExRequest(KMSG_MACSYMBOLQUOTES, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足或记录区不完整时返回错误。
func (obj *MACSymbolQuotes) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 26 {
		return fmt.Errorf("invalid mac symbol quotes response length: %d", len(data))
	}

	// 保存回显位图并据此还原激活字段定义，供后续按槽位解码。
	copy(obj.reply.FieldBitmap[:], data[:20])
	obj.reply.Total = binary.LittleEndian.Uint32(data[20:24])
	obj.reply.Count = binary.LittleEndian.Uint16(data[24:26])
	obj.reply.ActiveFields = activeMACDynamicFields(obj.reply.FieldBitmap)

	// 记录长度随激活字段数动态变化：68 字节身份区加每字段 4 字节数值区。
	rowLength := 68 + len(obj.reply.ActiveFields)*4
	pos := 26
	for i := uint16(0); i < obj.reply.Count; i++ {
		if pos+rowLength > len(data) {
			return fmt.Errorf("invalid mac symbol quote item %d", i)
		}
		row := data[pos : pos+rowLength]
		// 身份区固定布局：市场（0:2）、GBK 代码（2:24）、GBK 名称（24:68）。
		item := MACSymbolQuoteItem{
			Market: binary.LittleEndian.Uint16(row[:2]),
			Symbol: Utf8ToGbk(row[2:24]),
			Name:   Utf8ToGbk(row[24:68]),
			Values: make(map[string]any, len(obj.reply.ActiveFields)),
		}
		// 数值区自偏移 68 开始，按位图给出的字段顺序每字段 4 字节解码，同时写入主名与别名。
		fieldPos := 68
		for _, field := range obj.reply.ActiveFields {
			raw := row[fieldPos : fieldPos+4]
			value := decodeMACDynamicValue(field.Format, raw)
			item.Values[field.Name] = value
			for _, alias := range field.Aliases {
				item.Values[alias] = value
			}
			fieldPos += 4
		}
		obj.reply.Stocks = append(obj.reply.Stocks, item)
		pos += rowLength
	}

	return nil
}

// Response 返回解码完成的批量报价响应；未调用 ParseResponse 前内容为零值。
func (obj *MACSymbolQuotes) Response() *MACSymbolQuotesReply {
	return obj.reply
}
