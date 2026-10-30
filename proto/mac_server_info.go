package proto

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// MACServerInfo 查询服务端的交易日、交易时段、时间戳与市场参数等基础信息。
//
// 使用时先由 NewMACServerInfo 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACServerInfo struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACServerInfoRequest
	reply      *MACServerInfoReply
}

// MACServerInfoRequest 保存服务端信息请求的固定 68 字节负载。
type MACServerInfoRequest struct {
	Payload [68]byte
}

// MACTradingSession 表示一段交易时段，以距午夜的分钟数记录开盘与收盘时点，并换算为可读时间文本。
type MACTradingSession struct {
	OpenMinutes  uint16
	CloseMinutes uint16
	Open         string
	Close        string
}

// MACServerInfoReply 保存服务端信息响应的计数、标志、日期、两组交易时段、时间戳、市场参数与备查十六进制段。
type MACServerInfoReply struct {
	Count           uint16
	FlagsHex        string
	Tag             string
	Today           string
	TS1             uint32
	Sessions1       []MACTradingSession
	Sessions2       []MACTradingSession
	Flag            uint8
	LastTradingDay  string
	TS2             uint32
	LastTradingDay2 string
	TS3             uint32
	MarketParam1    uint32
	MarketParam2    uint32
	ExtraHex        string
}

// NewMACServerInfo 创建服务端信息查询对象；请求参数为空时使用内置默认负载。
func NewMACServerInfo(req *MACServerInfoRequest) *MACServerInfo {
	obj := &MACServerInfo{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACServerInfoRequest),
		reply:      new(MACServerInfoReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACSERVERINFO
	obj.request.Payload = defaultMACServerInfoPayload()
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACServerInfo) applyRequest(req *MACServerInfoRequest) {
	if req.Payload == ([68]byte{}) {
		req.Payload = defaultMACServerInfoPayload()
	}
	obj.request = req
}

// defaultMACServerInfoPayload 返回服务端信息请求的固定 68 字节默认负载，
// 仅在协议约定的偏移处填入标志与日期样例字节，其余补零。
func defaultMACServerInfoPayload() [68]byte {
	var payload [68]byte
	copy(payload[:4], []byte{0x04, 0x00, 0x2d, 0x31})
	copy(payload[12:16], []byte{0x00, 0x27, 0x06, 0x0e})
	return payload
}

// BuildRequest 把固定 68 字节负载封装为完整协议报文；负载长度固定，不存在序列化失败。
func (obj *MACServerInfo) BuildRequest() ([]byte, error) {
	return buildExRequest(KMSG_MACSERVERINFO, obj.request.Payload[:])
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足最小定长要求时返回错误。
func (obj *MACServerInfo) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 最小定长 87 字节，不足即判定为非法报文，避免后续切片越界。
	if len(data) < 87 {
		return fmt.Errorf("invalid mac server info response length: %d", len(data))
	}

	// 先读取条数；随后 8 字节标志位整体转十六进制备查。
	pos := 0
	obj.reply.Count = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2
	obj.reply.FlagsHex = hex.EncodeToString(data[pos : pos+8])
	pos += 8
	// 3 字节标签以空字符填充；其后 9 字节为协议保留区，直接跳过。
	obj.reply.Tag = strings.TrimRight(string(data[pos:pos+3]), "\x00")
	pos += 3
	pos += 9

	// 当日日期与时间戳，随后两组各 16 字节交易时段。
	obj.reply.Today = formatMACServerDate(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	obj.reply.TS1 = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	obj.reply.Sessions1 = parseMACTradingSessions(data[pos : pos+16])
	pos += 16
	obj.reply.Sessions2 = parseMACTradingSessions(data[pos : pos+16])
	pos += 16
	// 1 字节状态标志。
	obj.reply.Flag = data[pos]
	pos++
	// 上一交易日日期、时间戳、第二交易日日期、时间戳顺序排列。
	obj.reply.LastTradingDay = formatMACServerDate(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	obj.reply.TS2 = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	obj.reply.LastTradingDay2 = formatMACServerDate(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	obj.reply.TS3 = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	// 末尾两个市场参数；其后若仍有剩余字节，整体转十六进制备查。
	obj.reply.MarketParam1 = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	obj.reply.MarketParam2 = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	if pos < len(data) {
		obj.reply.ExtraHex = hex.EncodeToString(data[pos:])
	}
	return nil
}

// parseMACTradingSessions 把 16 字节时段缓冲按每 4 字节一组解析为开盘与收盘分钟数对，共至多 4 段；
// 每组两个两字节整数分别为开盘、收盘距午夜的分钟数，均按小端序读取。
func parseMACTradingSessions(data []byte) []MACTradingSession {
	sessions := make([]MACTradingSession, 0, 4)
	for i := 0; i+4 <= len(data); i += 4 {
		openMinutes := binary.LittleEndian.Uint16(data[i : i+2])
		closeMinutes := binary.LittleEndian.Uint16(data[i+2 : i+4])
		sessions = append(sessions, MACTradingSession{
			OpenMinutes:  openMinutes,
			CloseMinutes: closeMinutes,
			Open:         formatMACSessionTime(openMinutes),
			Close:        formatMACSessionTime(closeMinutes),
		})
	}
	return sessions
}

// formatMACSessionTime 将距午夜的分钟数格式化为可读时分文本。
func formatMACSessionTime(minutes uint16) string {
	return fmt.Sprintf("%d:%02d", minutes/60, minutes%60)
}

// formatMACServerDate 将打包日期整数拆出年、月、日并格式化为短横线分隔的日期文本。
func formatMACServerDate(raw uint32) string {
	return fmt.Sprintf("%04d-%02d-%02d", raw/10000, (raw%10000)/100, raw%100)
}

// Response 返回解码完成的服务端信息响应；未调用 ParseResponse 前内容为零值。
func (obj *MACServerInfo) Response() *MACServerInfoReply {
	return obj.reply
}
