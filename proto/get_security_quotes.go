package proto

import (
	"bytes"
	"encoding/binary"
)

// GetSecurityQuotes 表示一次批量获取盘口五档报价与完整行情快照协议的完整流程对象。
//
// 该对象按传入的证券列表批量返回实时行情。零值不可直接使用，必须通过
// NewGetSecurityQuotes 构造；对象绑定单次查询流程，不支持多协程并发复用。
type GetSecurityQuotes struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetSecurityQuotesRequest
	reply      *GetSecurityQuotesReply
}

// Stock 用市场代码与证券代码唯一标识一只待查询证券。
type Stock struct {
	Market uint8  // 市场代码。
	Code   string // 证券代码。
}

// GetSecurityQuotesRequest 是 GetSecurityQuotes 批量盘口报价查询使用的请求参数。
type GetSecurityQuotesRequest struct {
	StockList []Stock // 待查询的证券列表。
}

// GetSecurityQuotesReply 是 GetSecurityQuotes 批量盘口报价查询解析完成后的响应数据。
type GetSecurityQuotesReply struct {
	Count uint16          // 返回的证券数量。
	List  []SecurityQuote // 五档行情明细列表。
}

// SecurityQuote 表示一只证券的实时盘口行情快照，包含最新价、五档买卖价量等字段。
type SecurityQuote struct {
	Market     uint8   // 市场代码。
	Code       string  // 证券代码。
	Active1    uint16  // 活跃度字段 1，语义待确认。
	Close      float64 // 最新价，单位为元。
	Price      float64 // 当前价，通常与最新价一致。
	PreClose   float64 // 昨收价，单位为元。
	LastClose  float64 // 前收价别名，通常与昨收价一致。
	Open       float64 // 今开价，单位为元。
	High       float64 // 最高价，单位为元。
	Low        float64 // 最低价，单位为元。
	ServerTime string  // 服务端时间，格式为时:分:秒。
	NegPrice   float64 // 特殊价格字段，常见场景下可忽略，语义待确认。
	Vol        int     // 总成交量。
	CurVol     int     // 现量。
	Amount     float64 // 总成交额。
	SVol       int     // 外盘量或卖出量，口径语义待确认。
	BVol       int     // 内盘量或买入量，口径语义待确认。
	SAmount    int     // 上涨家数或卖出相关统计字段，语义待确认。
	OpenAmount int     // 开盘金额。
	BidLevels  []Level // 买盘五档。
	AskLevels  []Level // 卖盘五档。
	Bid1       float64 // 买一价。
	Ask1       float64 // 卖一价。
	BidVol1    int     // 买一量。
	AskVol1    int     // 卖一量。
	Bid2       float64 // 买二价。
	Ask2       float64 // 卖二价。
	BidVol2    int     // 买二量。
	AskVol2    int     // 卖二量。
	Bid3       float64 // 买三价。
	Ask3       float64 // 卖三价。
	BidVol3    int     // 买三量。
	AskVol3    int     // 卖三量。
	Bid4       float64 // 买四价。
	Ask4       float64 // 卖四价。
	BidVol4    int     // 买四量。
	AskVol4    int     // 卖四量。
	Bid5       float64 // 买五价。
	Ask5       float64 // 卖五价。
	BidVol5    int     // 买五量。
	AskVol5    int     // 卖五量。
	Unknown    int16   // 未确认字段，语义待确认。
	Rate       float64 // 涨跌幅或速率相关字段，语义待确认。
	Turnover   float64 // 换手率，由高层接口尽力估算补齐。
	Active2    uint16  // 活跃度字段 2，语义待确认。
}

// Level 表示盘口某一档的价格与挂单量。
type Level struct {
	Price float64 // 档位价格。
	Vol   int     // 档位挂单量。
}

// NewGetSecurityQuotes 构造盘口报价批量查询对象；`req` 为 `nil` 时使用空请求参数，返回的对象可直接调用 BuildRequest 与 ParseResponse。
func NewGetSecurityQuotes(req *GetSecurityQuotesRequest) *GetSecurityQuotes {
	obj := new(GetSecurityQuotes)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetSecurityQuotesRequest)
	obj.reply = new(GetSecurityQuotesReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_SECURITYQUOTES
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetSecurityQuotes) applyRequest(req *GetSecurityQuotesRequest) {
	obj.request = req
}

// BuildRequest 将待查询证券列表编码为协议报文；缓冲区写入失败时返回错误。
func (obj *GetSecurityQuotes) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 2 + uint16(len(obj.request.StockList)*7) + 10
	obj.reqHeader.PkgLen2 = 2 + uint16(len(obj.request.StockList)*7) + 10

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, obj.reqHeader)
	if err != nil {
		return nil, err
	}

	// 固定模式字 5。
	if err := binary.Write(buf, binary.LittleEndian, uint16(5)); err != nil {
		return nil, err
	}
	// 6 字节协议保留区，整体补零。
	if _, err := buf.Write(make([]byte, 6)); err != nil {
		return nil, err
	}
	// 小端序写入待查询证券个数。
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(obj.request.StockList))); err != nil {
		return nil, err
	}

	for _, stock := range obj.request.StockList {
		code := make([]byte, 6)
		copy(code, stock.Code)
		if err := binary.Write(buf, binary.LittleEndian, stock.Market); err != nil {
			return nil, err
		}
		if _, err := buf.Write(code); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// ParseResponse 解析服务端返回的盘口五档行情报文并填充响应对象；报文长度不足或解码失败时返回错误。
func (obj *GetSecurityQuotes) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header

	pos := 0
	// 前 2 字节为协议保留区，直接跳过。
	pos += 2
	if err := binary.Read(bytes.NewBuffer(data[pos:pos+2]), binary.LittleEndian, &obj.reply.Count); err != nil {
		return err
	}
	pos += 2

	for index := uint16(0); index < obj.reply.Count; index++ {
		ele := SecurityQuote{}
		// 市场 1 字节。
		if err := binary.Read(bytes.NewBuffer(data[pos:pos+1]), binary.LittleEndian, &ele.Market); err != nil {
			return err
		}
		pos++

		// 代码 6 字节，按 `GBK` 解码。
		var code [6]byte
		if err := binary.Read(bytes.NewBuffer(data[pos:pos+6]), binary.LittleEndian, &code); err != nil {
			return err
		}
		ele.Code = Utf8ToGbk(code[:])
		pos += 6

		// 活跃度字段 1，2 字节小端序。
		if err := binary.Read(bytes.NewBuffer(data[pos:pos+2]), binary.LittleEndian, &ele.Active1); err != nil {
			return err
		}
		pos += 2

		// 基准价与各价格差值；后续价位为基准价加差值。
		basePrice := getprice(data, &pos)
		preCloseDiff := getprice(data, &pos)
		openDiff := getprice(data, &pos)
		highDiff := getprice(data, &pos)
		lowDiff := getprice(data, &pos)
		serverTimeRaw := getprice(data, &pos)
		negPriceRaw := getprice(data, &pos)
		vol := getprice(data, &pos)
		curVol := getprice(data, &pos)
		// 成交额为 4 字节小端序单精度浮点数。
		amount := getfloat32(data, &pos)
		sVol := getprice(data, &pos)
		bVol := getprice(data, &pos)
		sAmount := getprice(data, &pos)
		openAmount := getprice(data, &pos)

		// 基准价加差值还原各价位，除以 100 得到元；前收价与昨收价同值。
		ele.Close = float64(basePrice) / 100.0
		ele.Price = ele.Close
		ele.PreClose = float64(basePrice+preCloseDiff) / 100.0
		ele.LastClose = ele.PreClose
		ele.Open = float64(basePrice+openDiff) / 100.0
		ele.High = float64(basePrice+highDiff) / 100.0
		ele.Low = float64(basePrice+lowDiff) / 100.0
		ele.ServerTime = formatServerTime(serverTimeRaw)
		ele.NegPrice = float64(negPriceRaw) / 100.0
		ele.Vol = vol
		ele.CurVol = curVol
		ele.Amount = amount
		ele.SVol = sVol
		ele.BVol = bVol
		ele.SAmount = sAmount
		ele.OpenAmount = openAmount

		// 五档买卖盘：每档买/卖价为基准价加档位差值，档位量独立读取。
		for i := 0; i < 5; i++ {
			bid := getprice(data, &pos) + basePrice
			ask := getprice(data, &pos) + basePrice
			bidVol := getprice(data, &pos)
			askVol := getprice(data, &pos)

			ele.BidLevels = append(ele.BidLevels, Level{
				Price: float64(bid) / 100.0,
				Vol:   bidVol,
			})
			ele.AskLevels = append(ele.AskLevels, Level{
				Price: float64(ask) / 100.0,
				Vol:   askVol,
			})
		}

		// 将档位列表拍平到买一至买五、卖一至卖五独立字段，便于直接访问。
		ele.Bid1 = ele.BidLevels[0].Price
		ele.Bid2 = ele.BidLevels[1].Price
		ele.Bid3 = ele.BidLevels[2].Price
		ele.Bid4 = ele.BidLevels[3].Price
		ele.Bid5 = ele.BidLevels[4].Price
		ele.Ask1 = ele.AskLevels[0].Price
		ele.Ask2 = ele.AskLevels[1].Price
		ele.Ask3 = ele.AskLevels[2].Price
		ele.Ask4 = ele.AskLevels[3].Price
		ele.Ask5 = ele.AskLevels[4].Price
		ele.BidVol1 = ele.BidLevels[0].Vol
		ele.BidVol2 = ele.BidLevels[1].Vol
		ele.BidVol3 = ele.BidLevels[2].Vol
		ele.BidVol4 = ele.BidLevels[3].Vol
		ele.BidVol5 = ele.BidLevels[4].Vol
		ele.AskVol1 = ele.AskLevels[0].Vol
		ele.AskVol2 = ele.AskLevels[1].Vol
		ele.AskVol3 = ele.AskLevels[2].Vol
		ele.AskVol4 = ele.AskLevels[3].Vol
		ele.AskVol5 = ele.AskLevels[4].Vol

		// 尾部未知 2 字节小端序字段，语义待确认。
		if err := binary.Read(bytes.NewBuffer(data[pos:pos+2]), binary.LittleEndian, &ele.Unknown); err != nil {
			return err
		}
		pos += 2
		// 4 字节协议保留区，整体跳过。
		pos += 4

		// 涨速为小端序无符号整数转有符号短整数，放大 100 倍存储，解析后除以 100。
		var riseSpeed int16
		if err := binary.Read(bytes.NewBuffer(data[pos:pos+2]), binary.LittleEndian, &riseSpeed); err != nil {
			return err
		}
		pos += 2
		ele.Rate = float64(riseSpeed) / 100.0
		// 活跃度字段 2，2 字节小端序。
		if err := binary.Read(bytes.NewBuffer(data[pos:pos+2]), binary.LittleEndian, &ele.Active2); err != nil {
			return err
		}
		pos += 2

		obj.reply.List = append(obj.reply.List, ele)
	}

	return nil
}

// Response 返回已解析完成的盘口行情响应数据；未调用 ParseResponse 时返回空响应对象。
func (obj *GetSecurityQuotes) Response() *GetSecurityQuotesReply {
	return obj.reply
}
