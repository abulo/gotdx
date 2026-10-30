package gotdx

// MACSortType 是 MAC 板块成分榜单的排序字段编号，作为请求的排序字段参数传入，指定结果按
// 哪一项指标排序；取值为协议约定的排序字段代码。
type MACSortType uint16

// MACSort* 板块成分排序字段编号，作为请求的排序字段参数传入，指定榜单按哪一项排序。
// 名称前缀对应排序维度：`Code` 代码、`Name` 名称、`Price` 最新价、`Vol` 成交量、
// `Amount` 成交额、`TotalAmount` 总成交额、`LastVolume` 现量、`Change` 涨跌额、
// `Industry` 行业、`ChangePct` 涨跌幅、`AmplitudePct` 振幅、`ShortTurnoverPct` 短换手、
// `VolSpeedPct` 量涨速、`MainNetAmount` 主力净流入、`MainNetRatio` 主力净比、
// `AuctionLimitBuy` / `AuctionLimitSell` 竞价买入上限 / 卖出下限、`LimitUpTime` /
// `LimitDownTime` 涨停 / 跌停时间、`DrawdownPct` 回撤、`YTDPct` 年初至今、
// `ConsecutiveUpDays` 连涨天数、`MTDPct` 月初至今、`PETTM` / `PEStatic` 市盈率滚动 / 静态；
// `AttackPct` 按攻击幅度排序，具体语义待确认。
const (
	MACSortCode              MACSortType = 0x00
	MACSortName              MACSortType = 0x01
	MACSortPrice             MACSortType = 0x06
	MACSortVol               MACSortType = 0x07
	MACSortAmount            MACSortType = 0x08
	MACSortTotalAmount       MACSortType = 0x0a
	MACSortLastVolume        MACSortType = 0x0b
	MACSortChange            MACSortType = 0x0c
	MACSortIndustry          MACSortType = 0x0d
	MACSortChangePct         MACSortType = 0x0e
	MACSortAmplitudePct      MACSortType = 0x0f
	MACSortShortTurnoverPct  MACSortType = 0xcc
	MACSortVolSpeedPct       MACSortType = 0xd0
	MACSortChange5DPct       MACSortType = 0xd1
	MACSortChange10DPct      MACSortType = 0xd2
	MACSortMainNetAmount     MACSortType = 0xd4
	MACSortMainNetRatio      MACSortType = 0xd7
	MACSortAuctionLimitBuy   MACSortType = 0x102
	MACSortAuctionLimitSell  MACSortType = 0x103
	MACSortLimitUpTime       MACSortType = 0x108
	MACSortLimitDownTime     MACSortType = 0x109
	MACSortDrawdownPct       MACSortType = 0x11e
	MACSortAttackPct         MACSortType = 0x11f
	MACSortYTDPct            MACSortType = 0x121
	MACSortConsecutiveUpDays MACSortType = 0x138
	MACSortChange3DPct       MACSortType = 0x139
	MACSortDividendYieldRate MACSortType = 0x13a
	MACSortChange20DPct      MACSortType = 0x172
	MACSortChange60DPct      MACSortType = 0x173
	MACSortPrevChangePct     MACSortType = 0x17e
	MACSortMTDPct            MACSortType = 0x19c
	MACSortChange1YPct       MACSortType = 0x19d
	MACSortPETTM             MACSortType = 0x3ee
	MACSortPEStatic          MACSortType = 0x3ef
)

// MACSortOrder 是 MAC 板块成分榜单的排序方向编号，作为请求的排序方向参数传入，控制排序结果
// 按升序还是降序返回。
type MACSortOrder uint8

// MACSortOrder* 排序方向编号，作为请求的排序方向参数传入：`None` 不指定排序方向，`Desc`
// 降序，`Asc` 升序。
const (
	MACSortOrderNone MACSortOrder = 0
	MACSortOrderDesc MACSortOrder = 1
	MACSortOrderAsc  MACSortOrder = 2
)

// MACFilterType 是 MAC 板块成分的排除条件，每个条件独占一个二进制位；多个条件按位或叠加后
// 写入请求位图的过滤字节，命中条件的标的将被排除。
type MACFilterType uint8

// MACFilter* 成分股排除条件位图，可按位叠加组合，命中条件的标的将被排除：`New` 新股、
// `KCB` 科创板、`ST` 风险警示股、`GEM` 创业板、`HKConnect` 港股通、`BJ` 北交所、
// `Approval` 核准制、`Registration` 注册制。
const (
	MACFilterNew          MACFilterType = 1
	MACFilterKCB          MACFilterType = 2
	MACFilterST           MACFilterType = 4
	MACFilterGEM          MACFilterType = 8
	MACFilterHKConnect    MACFilterType = 16
	MACFilterBJ           MACFilterType = 32
	MACFilterApproval     MACFilterType = 64
	MACFilterRegistration MACFilterType = 128
)

// MACFilterFlags 将多个排除条件合并为一个协议位图字节。
//
// 各条件本身即 2 的幂位掩码，按位或叠加后得到可放入请求过滤字节的组合值；不传条件时返回 0。
func MACFilterFlags(filters ...MACFilterType) uint8 {
	var flags uint8
	for _, filter := range filters {
		// 各条件独占一位，按位或合并，不同条件的位互不覆盖。
		flags |= uint8(filter)
	}
	return flags
}

// macBoardMembersQuotesControlExtended 是请求位图第 19 字节的扩展控制位，固定置 1，
// 与动态字段扩展请求配套使用。
const macBoardMembersQuotesControlExtended byte = 1

// MACBoardMembersQuotesRequestBitmap 在动态字段位图基础上拼装成分股动态字段请求位图：
// 第 17 字节写入合并后的排除条件掩码，第 19 字节置扩展控制位，其余字段位保持不变。
//
// 入参位图按值复制后修改，不会影响调用方传入的原位图。
func MACBoardMembersQuotesRequestBitmap(fieldBitmap [20]byte, filters ...MACFilterType) [20]byte {
	// 协议约定第 17 字节存放成分股排除条件的组合掩码。
	fieldBitmap[17] = MACFilterFlags(filters...)
	// 第 19 字节按位或置固定扩展控制位，保留该字节上已有的其他位。
	fieldBitmap[19] |= macBoardMembersQuotesControlExtended
	return fieldBitmap
}
