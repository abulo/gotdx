package proto

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"time"
)

// ExLogin 表示扩展市场服务器登录握手协议，发送固定握手报文并解析服务器时间、名称与地址等信息。
type ExLogin struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	reply      *ExLoginReply
	contentHex string
}

// ExLoginReply 保存解码完成的登录握手响应结果。
type ExLoginReply struct {
	DateTime   string   // 服务器当前时间。
	ServerName string   // 服务器名称。
	Desc       string   // 服务器描述信息。
	IP         string   // 服务器地址。
	Unknown    []string // 协议保留字段组，语义待确认。
}

// NewExLogin 构造登录握手协议实例，内置协议固定握手报文并完成报文头与消息号设置。
func NewExLogin() *ExLogin {
	obj := &ExLogin{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		reply:      new(ExLoginReply),
		contentHex: "e5bb1c2fafe525941f32c6e5d53dfb415b734cc9cdbf0ac92021bfdd1eb06d22d008884c1611cb1378f6abd824d899d21f32c6e5d53dfb411f32c6e5d53dfb41a9325ac935dc0837335a16e4ce17c1bb",
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXLOGIN
	return obj
}

// BuildRequest 把内置固定握手报文解码为字节并编码为扩展协议请求字节流；十六进制格式非法时返回非空错误。
func (obj *ExLogin) BuildRequest() ([]byte, error) {
	// 解码内置固定握手报文，十六进制格式非法时直接返回错误。
	payload, err := hex.DecodeString(obj.contentHex)
	if err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_EXLOGIN, payload)
}

// ParseResponse 从服务端响应字节流解码登录握手信息并填充响应；响应长度不足时返回非空错误。
func (obj *ExLogin) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 握手响应固定区至少需要 299 字节。
	if len(data) < 299 {
		return fmt.Errorf("invalid ex login response length: %d", len(data))
	}

	// 服务器时间各分量按固定偏移分别取出，年份为小端 `u16`，其余分量为单字节。
	year := binary.LittleEndian.Uint16(data[53:55])
	month := int(data[55])
	day := int(data[56])
	minute := int(data[57])
	hour := int(data[58])
	second := int(data[60])

	obj.reply.DateTime = time.Date(int(year), time.Month(month), day, hour, minute, second, 0, time.Local).Format("2006-01-02 15:04:05")
	obj.reply.ServerName = Utf8ToGbk(data[61:82])
	obj.reply.Desc = Utf8ToGbk(data[93:244])
	obj.reply.IP = Utf8ToGbk(data[247:299])
	// 零散保留字段按各自位宽读出后格式化为字符串，语义待确认。
	obj.reply.Unknown = []string{
		fmt.Sprintf("%g", float64(math.Float32frombits(binary.LittleEndian.Uint32(data[82:86])))),
		fmt.Sprintf("%d", data[86]),
		fmt.Sprintf("%d", binary.LittleEndian.Uint16(data[87:89])),
		fmt.Sprintf("%d", binary.LittleEndian.Uint16(data[89:91])),
		fmt.Sprintf("%d", binary.LittleEndian.Uint16(data[91:93])),
		fmt.Sprintf("%d", data[244]),
		fmt.Sprintf("%d", data[245]),
		fmt.Sprintf("%d", data[246]),
	}
	return nil
}

// Response 返回解码完成的登录握手响应对象。
func (obj *ExLogin) Response() *ExLoginReply {
	return obj.reply
}

// ExServerInfo 表示扩展市场服务器信息查询协议，返回网络延迟、版本号、签名与当前时间等信息；
// 该协议无请求参数体。
type ExServerInfo struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	reply      *ExServerInfoReply
}

// ExServerInfoReply 保存解码完成的服务器信息响应结果。
type ExServerInfoReply struct {
	Delay       uint32 // 网络延迟（毫秒），语义待确认。
	Info        string // 服务器附加信息。
	Version     string // 服务器版本号。
	ServerSign  string // 服务器签名。
	ServerSign2 string // 第二服务器签名，语义待确认。
	TimeNow     string // 服务器当前时间。
	ServerName  string // 服务器名称。
	Name        string // 服务器显示名称。
}

// NewExServerInfo 构造服务器信息查询协议实例。
func NewExServerInfo() *ExServerInfo {
	obj := &ExServerInfo{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		reply:      new(ExServerInfoReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_EXSERVERINFO
	return obj
}

// BuildRequest 构造服务器信息查询请求字节流；该协议无载荷，仅由协议头与消息号组成。
func (obj *ExServerInfo) BuildRequest() ([]byte, error) {
	return buildExRequest(KMSG_EXSERVERINFO, nil)
}

// ParseResponse 从服务端响应字节流解码服务器信息并填充响应；响应长度不足时返回非空错误。
func (obj *ExServerInfo) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 服务器信息固定区至少需要 327 字节。
	if len(data) < 327 {
		return fmt.Errorf("invalid ex server info response length: %d", len(data))
	}

	obj.reply.Delay = binary.LittleEndian.Uint32(data[:4])
	obj.reply.Info = Utf8ToGbk(data[16:41])
	obj.reply.Version = Utf8ToGbk(data[41:70])
	obj.reply.ServerSign = Utf8ToGbk(data[117:130])
	obj.reply.ServerSign2 = Utf8ToGbk(data[240:253])

	// 日期编号 `YYYYMMDD` 与时间编号 `HHMMSS` 分别拆分出年月日与时分秒。
	dateNow := binary.LittleEndian.Uint32(data[80:84])
	timeNow := binary.LittleEndian.Uint32(data[84:88])
	year := int(dateNow / 10000)
	month := int((dateNow % 10000) / 100)
	day := int(dateNow % 100)
	hour := int(timeNow / 10000)
	minute := int((timeNow % 10000) / 100)
	second := int(timeNow % 100)
	obj.reply.TimeNow = time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local).Format("2006-01-02 15:04:05")
	obj.reply.Name = Utf8ToGbk(data[159:189])
	obj.reply.ServerName = obj.reply.Name
	return nil
}

// Response 返回解码完成的服务器信息响应对象。
func (obj *ExServerInfo) Response() *ExServerInfoReply {
	return obj.reply
}
