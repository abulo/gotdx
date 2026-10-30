package gotdx

// MACFieldSelector 是参与构造 MAC 动态字段位图的选择项接口。
//
// 单个字段位（`MACFieldBit`）与一组字段预设（`MACPresetField`）均实现该接口；调用方将
// 若干选择项传给 `MACFieldBitmap`，由其组合出最终位图。位图中被置位的字段才会在服务端
// 响应中返回。
type MACFieldSelector interface {
	// applyMACFieldBitmap 把本选择项覆盖的字段位写入目标位图。
	applyMACFieldBitmap(*[20]byte)
}

// MACFieldBit 表示 MAC 动态字段位图中的单个字段位，底层为协议位号，有效范围为 0–159
// （对应 20 字节位图的 160 个二进制位）。
//
// 位号与 `proto/mac_board_members_dynamic.go` 中的字段映射表一一对应；超出位图容量的位号
// 在置位时会被忽略，不会产生越界写入。
type MACFieldBit uint8

// MACField* 位图字段：MAC 动态字段位图中可请求的行情字段，位号与
// `proto/mac_board_members_dynamic.go` 的字段映射表对应；位图中置位后，服务端才会返回
// 对应字段。名称自明的开高低收、量额、盘口、涨跌等字段不再逐条说明；含义不直观、存在双义
// 或别名的字段如下，标注“语义待确认”者协议含义未在映射表中确证，不应据此推断协议细节。
//
// 含义不直观或存在双义的字段：
//   - `SecurityTypePrice` 证券类型价（别名 `action_price`，语义待确认）
//   - `LotSizeInfo` 每手信息（映射表标注未确定，语义待确认）
//   - `PriceDecimalInfo` 价格精度标志（语义待确认）
//   - `LotSize` 所属地区板块（A 股）/ 每手股数（港股），含义随品种双义
//   - `PETTMVolRelated` 前参考价（美股适用）
//   - `ExPricePlaceholder` 前金额参考占位（别名 `close_placeholder`）
//   - `CirculatingCapitalZ` 流通股本（单位万股，语义待确认）
//   - `PercentBase` 百分比基底（语义待确认）
//   - `UnknownCode` 未知代码，少部分为 6 位数字，语义待确认
//   - `RetailNetAmount` 散户单增比（映射表原文，语义待确认）
//   - `MainNet10DAmount` 近十日主买金额（映射表标注待确定）
//   - `DDF` 资金指标，具体含义语义待确认
//   - `AuctionVolRatio` 竞价昨比（映射表原文，语义待确认）
//   - `RecentIndicator` 近日指标提示，语义待确认
//   - `HighlightCount` 亮点数（别名 `StockRating`，个股评级）
//   - `LimitUpCount`、`LimitDownCount`、`UpCount`、`DownCount` 在板块场景分别为涨停、
//     跌停、上涨、下跌家数；在个股场景复用为买二、卖二、买五、卖五量
//
// 块末若干常量为历史遗留别名，指向同一字段位，保留以兼容旧调用：`SomeBitmap` 对应
// `StockTagFlags`、`PreIPOV` 对应 `PreIOPV`、`IPOV` 对应 `IOPV`、`GEMStarInfo` 对应
// `AfterHoursVolume`、`DividendYieldPct` 对应 `DividendYieldRate`、`ConstantNegOne`
// 对应 `SafetyScore`、`StockRating` 对应 `HighlightCount`、`Bid2Volume` 对应
// `LimitUpCount`、`Ask2Volume` 对应 `LimitDownCount`、`Bid5Volume` 对应 `UpCount`、
// `Ask5Volume` 对应 `DownCount`。
const (
	MACFieldPreClose            MACFieldBit = 0x0
	MACFieldOpen                MACFieldBit = 0x1
	MACFieldHigh                MACFieldBit = 0x2
	MACFieldLow                 MACFieldBit = 0x3
	MACFieldClose               MACFieldBit = 0x4
	MACFieldVol                 MACFieldBit = 0x5
	MACFieldVolRatio            MACFieldBit = 0x6
	MACFieldAmount              MACFieldBit = 0x7
	MACFieldInsideVolume        MACFieldBit = 0x8
	MACFieldOutsideVolume       MACFieldBit = 0x9
	MACFieldTotalShares         MACFieldBit = 0xa
	MACFieldFloatShares         MACFieldBit = 0xb
	MACFieldEPS                 MACFieldBit = 0xc
	MACFieldNetAssets           MACFieldBit = 0xd
	MACFieldSecurityTypePrice   MACFieldBit = 0xe
	MACFieldTotalMarketCapAb    MACFieldBit = 0xf
	MACFieldPEDynamic           MACFieldBit = 0x10
	MACFieldBidPrice            MACFieldBit = 0x11
	MACFieldAskPrice            MACFieldBit = 0x12
	MACFieldServerUpdateDate    MACFieldBit = 0x13
	MACFieldServerUpdateTime    MACFieldBit = 0x14
	MACFieldLotSizeInfo         MACFieldBit = 0x15
	MACFieldBoardStrength       MACFieldBit = 0x16
	MACFieldDividendYield       MACFieldBit = 0x17
	MACFieldBidVolume           MACFieldBit = 0x18
	MACFieldAskVolume           MACFieldBit = 0x19
	MACFieldLastVolume          MACFieldBit = 0x1a
	MACFieldTurnover            MACFieldBit = 0x1b
	MACFieldIndustry            MACFieldBit = 0x1c
	MACFieldIndustryChangeUp    MACFieldBit = 0x1d
	MACFieldStockTagFlags       MACFieldBit = 0x1e
	MACFieldDecimalPoint        MACFieldBit = 0x1f
	MACFieldBuyPriceLimit       MACFieldBit = 0x20
	MACFieldSellPriceLimit      MACFieldBit = 0x21
	MACFieldPriceDecimalInfo    MACFieldBit = 0x22
	MACFieldLotSize             MACFieldBit = 0x23
	MACFieldPreIOPV             MACFieldBit = 0x24
	MACFieldSpeedPct            MACFieldBit = 0x25
	MACFieldAvgPrice            MACFieldBit = 0x26
	MACFieldIOPV                MACFieldBit = 0x27
	MACFieldPETTMVolRelated     MACFieldBit = 0x28
	MACFieldExPricePlaceholder  MACFieldBit = 0x29
	MACFieldOperatingRevenue    MACFieldBit = 0x2a
	MACFieldFlagKCB             MACFieldBit = 0x2b
	MACFieldFlagBJ              MACFieldBit = 0x2c
	MACFieldCirculatingCapitalZ MACFieldBit = 0x2d
	MACFieldAfterHoursVolume    MACFieldBit = 0x2e
	MACFieldPETTM               MACFieldBit = 0x30
	MACFieldPEStatic            MACFieldBit = 0x31
	MACFieldIndexMetric         MACFieldBit = 0x37
	MACFieldMainNetAmount       MACFieldBit = 0x38
	MACFieldBidAskRatio         MACFieldBit = 0x39
	MACFieldNonIndexFlag        MACFieldBit = 0x3a
	MACFieldChange20DPct        MACFieldBit = 0x3b
	MACFieldYTDPct              MACFieldBit = 0x3c
	MACFieldStockClassCode      MACFieldBit = 0x3e
	MACFieldPercentBase         MACFieldBit = 0x3f
	MACFieldMTDPct              MACFieldBit = 0x40
	MACFieldChange1yPct         MACFieldBit = 0x41
	MACFieldPrevChangePct       MACFieldBit = 0x42
	MACFieldChange3DPct         MACFieldBit = 0x43
	MACFieldChange60DPct        MACFieldBit = 0x44
	MACFieldChange5DPct         MACFieldBit = 0x45
	MACFieldChange10DPct        MACFieldBit = 0x46
	MACFieldPrev2ChangePct      MACFieldBit = 0x47
	MACFieldBid2Price           MACFieldBit = 0x48
	MACFieldAsk2Price           MACFieldBit = 0x49
	MACFieldAHCode              MACFieldBit = 0x4a
	MACFieldUnknownCode         MACFieldBit = 0x4b
	MACFieldOpenAmount          MACFieldBit = 0x57
	MACFieldAnnualLimitUpDays   MACFieldBit = 0x58
	MACFieldActivity            MACFieldBit = 0x59
	MACFieldDividendYieldRate   MACFieldBit = 0x5b
	MACFieldConsecutiveUpDays   MACFieldBit = 0x5c
	MACFieldLimitUpCount        MACFieldBit = 0x5d
	MACFieldLimitDownCount      MACFieldBit = 0x5e
	MACFieldIndustrySub         MACFieldBit = 0x5f
	MACFieldAuctionBuyLimit     MACFieldBit = 0x66
	MACFieldAuctionSellLimit    MACFieldBit = 0x67
	MACFieldVolSpeedPct         MACFieldBit = 0x68
	MACFieldShortTurnoverPct    MACFieldBit = 0x69
	MACFieldAmount2m            MACFieldBit = 0x6a
	MACFieldMainNetAmountCopy   MACFieldBit = 0x6b
	MACFieldMainNetRatio        MACFieldBit = 0x6c
	MACFieldRetailNetAmount     MACFieldBit = 0x6d
	MACFieldMainNet5mAmount     MACFieldBit = 0x6e
	MACFieldMainNet3DAmount     MACFieldBit = 0x6f
	MACFieldMainNet5DAmount     MACFieldBit = 0x70
	MACFieldMainNet10DAmount    MACFieldBit = 0x71
	MACFieldMainBuyNetAmount    MACFieldBit = 0x72
	MACFieldDDX                 MACFieldBit = 0x73
	MACFieldDDY                 MACFieldBit = 0x74
	MACFieldDDZ                 MACFieldBit = 0x75
	MACFieldDDF                 MACFieldBit = 0x76
	MACFieldStockFlagA          MACFieldBit = 0x77
	MACFieldStockFlagB          MACFieldBit = 0x78
	MACFieldAuctionVolRatio     MACFieldBit = 0x7a
	MACFieldPrevAmount          MACFieldBit = 0x7b
	MACFieldRecentIndicator     MACFieldBit = 0x7d
	MACFieldBid3Price           MACFieldBit = 0x80
	MACFieldBid4Price           MACFieldBit = 0x81
	MACFieldBid5Price           MACFieldBit = 0x82
	MACFieldAsk3Price           MACFieldBit = 0x83
	MACFieldAsk4Price           MACFieldBit = 0x84
	MACFieldAsk5Price           MACFieldBit = 0x85
	MACFieldBid3Volume          MACFieldBit = 0x86
	MACFieldBid4Volume          MACFieldBit = 0x87
	MACFieldUpCount             MACFieldBit = 0x88
	MACFieldAsk3Volume          MACFieldBit = 0x89
	MACFieldAsk4Volume          MACFieldBit = 0x8a
	MACFieldDownCount           MACFieldBit = 0x8b
	MACFieldBidAskDiff          MACFieldBit = 0x8c
	MACFieldChangeUpType        MACFieldBit = 0x8d
	MACFieldSafetyScore         MACFieldBit = 0x8e
	MACFieldHighlightCount      MACFieldBit = 0x8f
	MACFieldChangeAt1000        MACFieldBit = 0x90
	MACFieldChangeAt1030        MACFieldBit = 0x91
	MACFieldChangeAt1100        MACFieldBit = 0x92
	MACFieldChangeAt1130        MACFieldBit = 0x93
	MACFieldChangeAt1330        MACFieldBit = 0x94
	MACFieldChangeAt1400        MACFieldBit = 0x95
	MACFieldChangeAt1430        MACFieldBit = 0x96
	MACFieldSomeBitmap          MACFieldBit = MACFieldStockTagFlags
	MACFieldPreIPOV             MACFieldBit = MACFieldPreIOPV
	MACFieldIPOV                MACFieldBit = MACFieldIOPV
	MACFieldGEMStarInfo         MACFieldBit = MACFieldAfterHoursVolume
	MACFieldDividendYieldPct    MACFieldBit = MACFieldDividendYieldRate
	MACFieldConstantNegOne      MACFieldBit = MACFieldSafetyScore
	MACFieldStockRating         MACFieldBit = MACFieldHighlightCount
	MACFieldBid2Volume          MACFieldBit = MACFieldLimitUpCount
	MACFieldAsk2Volume          MACFieldBit = MACFieldLimitDownCount
	MACFieldBid5Volume          MACFieldBit = MACFieldUpCount
	MACFieldAsk5Volume          MACFieldBit = MACFieldDownCount
)

// applyMACFieldBitmap 将本字段位置入目标位图；位号超出位图容量时静默忽略，避免构造
// 预设位集时传入越位常量造成越界写入。
func (bit MACFieldBit) applyMACFieldBitmap(bitmap *[20]byte) {
	// 位图固定 20 字节共 160 位，越界位号直接丢弃，保证后续按位运算不会访问越界字节。
	if int(bit) >= len(bitmap)*8 {
		return
	}
	// 按 `bit/8` 定位所在字节、`bit%8` 定位字节内的位，先取移位掩码再按位或完成置位。
	bitmap[int(bit)/8] |= 1 << uint(int(bit)%8)
}

// MACPresetField 表示一组常用 MAC 动态字段预设：一个预设编号对应一组固定字段位，调用方
// 无需逐位列举字段，即可通过 `MACFieldBitmap` 选中整组字段。
type MACPresetField uint8

// MACPreset* 常用字段预设：一次性选中一组 MAC 动态字段，便于直接构造字段位图。
//   - `None` 不选择任何字段；`OHLC` 为开、高、低、收四个价格字段
//   - `Basic` 基础行情（开高低收、昨收、成交量）；`Quote` 盘口报价（买卖价、买卖量、现量）
//   - `Volume` 量额换手（成交量、成交额、换手率、量比）；`Fundamental` 基本面（总股本、流通股、每股收益、净资产）
//   - `Enhanced` 增强行情（行情、流通股、活跃度）；`AHCode` 含 A/H 股代码与行业等辅助信息
//   - `BoardStats` 板块涨跌统计（涨停、跌停、上涨、下跌家数）；`Handicap` 五档盘口
//   - `Common` 常用字段集合，覆盖普通行情查询所需字段；`Debug` 调试用，将位图全部位置 1；`All` 选中全部已定义字段
const (
	MACPresetNone        MACPresetField = 0
	MACPresetOHLC        MACPresetField = 1
	MACPresetBasic       MACPresetField = 2
	MACPresetQuote       MACPresetField = 3
	MACPresetVolume      MACPresetField = 4
	MACPresetFundamental MACPresetField = 5
	MACPresetEnhanced    MACPresetField = 6
	MACPresetAHCode      MACPresetField = 7
	MACPresetBoardStats  MACPresetField = 8
	MACPresetHandicap    MACPresetField = 9
	MACPresetCommon      MACPresetField = 10
	MACPresetDebug       MACPresetField = 11
	MACPresetAll         MACPresetField = 12
)

// applyMACFieldBitmap 按预设编号把内置字段位集展开写入位图：`None` 不写任何位，`Debug`
// 将整份位图全部置 1，其余预设遍历各自字段位列表逐个置位。
func (preset MACPresetField) applyMACFieldBitmap(bitmap *[20]byte) {
	switch preset {
	case MACPresetNone:
		return
	case MACPresetOHLC:
		for _, bit := range []MACFieldBit{MACFieldOpen, MACFieldHigh, MACFieldLow, MACFieldClose} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetBasic:
		for _, bit := range []MACFieldBit{MACFieldOpen, MACFieldHigh, MACFieldLow, MACFieldClose, MACFieldPreClose, MACFieldVol} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetQuote:
		for _, bit := range []MACFieldBit{MACFieldBidPrice, MACFieldAskPrice, MACFieldBidVolume, MACFieldAskVolume, MACFieldLastVolume} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetVolume:
		for _, bit := range []MACFieldBit{MACFieldVol, MACFieldAmount, MACFieldTurnover, MACFieldVolRatio} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetFundamental:
		for _, bit := range []MACFieldBit{MACFieldTotalShares, MACFieldFloatShares, MACFieldEPS, MACFieldNetAssets} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetEnhanced:
		for _, bit := range []MACFieldBit{MACFieldOpen, MACFieldHigh, MACFieldLow, MACFieldClose, MACFieldVol, MACFieldFloatShares, MACFieldActivity} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetAHCode:
		for _, bit := range []MACFieldBit{MACFieldOpen, MACFieldHigh, MACFieldLow, MACFieldClose, MACFieldVol, MACFieldAHCode, MACFieldLotSize, MACFieldIndustry} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetBoardStats:
		for _, bit := range []MACFieldBit{MACFieldLimitUpCount, MACFieldLimitDownCount, MACFieldUpCount, MACFieldDownCount} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetHandicap:
		for _, bit := range []MACFieldBit{MACFieldBidPrice, MACFieldBid2Price, MACFieldBid3Price, MACFieldBid4Price, MACFieldBid5Price, MACFieldAskPrice, MACFieldAsk2Price, MACFieldAsk3Price, MACFieldAsk4Price, MACFieldAsk5Price, MACFieldBidVolume, MACFieldLimitUpCount, MACFieldBid3Volume, MACFieldBid4Volume, MACFieldUpCount, MACFieldAskVolume, MACFieldLimitDownCount, MACFieldAsk3Volume, MACFieldAsk4Volume, MACFieldDownCount} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetCommon:
		for _, bit := range []MACFieldBit{MACFieldPreClose, MACFieldOpen, MACFieldHigh, MACFieldLow, MACFieldClose, MACFieldVol, MACFieldVolRatio, MACFieldAmount, MACFieldTotalShares, MACFieldFloatShares, MACFieldEPS, MACFieldNetAssets, MACFieldSecurityTypePrice, MACFieldTotalMarketCapAb, MACFieldPEDynamic, MACFieldLotSizeInfo, MACFieldDividendYield, MACFieldLastVolume, MACFieldTurnover, MACFieldStockTagFlags, MACFieldDecimalPoint, MACFieldBuyPriceLimit, MACFieldSellPriceLimit, MACFieldPriceDecimalInfo, MACFieldLotSize, MACFieldPreIOPV, MACFieldSpeedPct, MACFieldFlagKCB, MACFieldPETTM, MACFieldPEStatic, MACFieldMainNetAmount, MACFieldVolSpeedPct, MACFieldShortTurnoverPct, MACFieldCirculatingCapitalZ} {
			bit.applyMACFieldBitmap(bitmap)
		}
	case MACPresetDebug:
		// 调试场景需要请求全部字段，故将 20 字节全部填为 `0xff`，等价于 160 位全部置 1。
		for i := range bitmap {
			bitmap[i] = 0xff
		}
	case MACPresetAll:
		for _, bit := range []MACFieldBit{MACFieldPreClose, MACFieldOpen, MACFieldHigh, MACFieldLow, MACFieldClose, MACFieldVol, MACFieldVolRatio, MACFieldAmount, MACFieldInsideVolume, MACFieldOutsideVolume, MACFieldTotalShares, MACFieldFloatShares, MACFieldEPS, MACFieldNetAssets, MACFieldSecurityTypePrice, MACFieldTotalMarketCapAb, MACFieldPEDynamic, MACFieldBidPrice, MACFieldAskPrice, MACFieldServerUpdateDate, MACFieldServerUpdateTime, MACFieldLotSizeInfo, MACFieldBoardStrength, MACFieldDividendYield, MACFieldBidVolume, MACFieldAskVolume, MACFieldLastVolume, MACFieldTurnover, MACFieldIndustry, MACFieldIndustryChangeUp, MACFieldStockTagFlags, MACFieldDecimalPoint, MACFieldBuyPriceLimit, MACFieldSellPriceLimit, MACFieldPriceDecimalInfo, MACFieldLotSize, MACFieldPreIOPV, MACFieldSpeedPct, MACFieldAvgPrice, MACFieldIOPV, MACFieldPETTMVolRelated, MACFieldExPricePlaceholder, MACFieldOperatingRevenue, MACFieldFlagKCB, MACFieldFlagBJ, MACFieldCirculatingCapitalZ, MACFieldAfterHoursVolume, MACFieldPETTM, MACFieldPEStatic, MACFieldIndexMetric, MACFieldMainNetAmount, MACFieldBidAskRatio, MACFieldNonIndexFlag, MACFieldChange20DPct, MACFieldYTDPct, MACFieldStockClassCode, MACFieldPercentBase, MACFieldMTDPct, MACFieldChange1yPct, MACFieldPrevChangePct, MACFieldChange3DPct, MACFieldChange60DPct, MACFieldChange5DPct, MACFieldChange10DPct, MACFieldPrev2ChangePct, MACFieldBid2Price, MACFieldAsk2Price, MACFieldAHCode, MACFieldUnknownCode, MACFieldOpenAmount, MACFieldAnnualLimitUpDays, MACFieldActivity, MACFieldDividendYieldRate, MACFieldConsecutiveUpDays, MACFieldLimitUpCount, MACFieldLimitDownCount, MACFieldIndustrySub, MACFieldAuctionBuyLimit, MACFieldAuctionSellLimit, MACFieldVolSpeedPct, MACFieldShortTurnoverPct, MACFieldAmount2m, MACFieldMainNetAmountCopy, MACFieldMainNetRatio, MACFieldRetailNetAmount, MACFieldMainNet5mAmount, MACFieldMainNet3DAmount, MACFieldMainNet5DAmount, MACFieldMainNet10DAmount, MACFieldMainBuyNetAmount, MACFieldDDX, MACFieldDDY, MACFieldDDZ, MACFieldDDF, MACFieldStockFlagA, MACFieldStockFlagB, MACFieldAuctionVolRatio, MACFieldPrevAmount, MACFieldRecentIndicator, MACFieldBid3Price, MACFieldBid4Price, MACFieldBid5Price, MACFieldAsk3Price, MACFieldAsk4Price, MACFieldAsk5Price, MACFieldBid3Volume, MACFieldBid4Volume, MACFieldUpCount, MACFieldAsk3Volume, MACFieldAsk4Volume, MACFieldDownCount, MACFieldBidAskDiff, MACFieldChangeUpType, MACFieldSafetyScore, MACFieldHighlightCount, MACFieldChangeAt1000, MACFieldChangeAt1030, MACFieldChangeAt1100, MACFieldChangeAt1130, MACFieldChangeAt1330, MACFieldChangeAt1400, MACFieldChangeAt1430} {
			bit.applyMACFieldBitmap(bitmap)
		}
	}
}

// MACFieldBitmap 根据传入的字段位与预设组合构造 20 字节 MAC 动态字段位图。
//
// 选择项按传入顺序依次叠加；值为 `nil` 的选择项会被跳过。返回值是独立的 20 字节副本，
// 调用方可自由修改；函数不持有共享状态，可并发调用。
func MACFieldBitmap(selectors ...MACFieldSelector) [20]byte {
	var bitmap [20]byte
	for _, selector := range selectors {
		// 跳过空选择项，兼容调用方混入空预设的场景，避免空指针调用。
		if selector == nil {
			continue
		}
		selector.applyMACFieldBitmap(&bitmap)
	}
	return bitmap
}

// DefaultMACBoardMembersQuotesFieldBitmap 返回成分股动态字段报价接口使用的默认字段位图。
//
// 返回固定的 20 字节字面量，置 1 的位对应默认启用的动态字段；调用方可在此基础上按需
// 增删字段位。
func DefaultMACBoardMembersQuotesFieldBitmap() [20]byte {
	return [20]byte{
		0xff, 0xfc, 0xe1, 0xcc, 0x3f, 0x08, 0x03, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}

// FullMACBoardMembersQuotesFieldBitmap 返回 20 字节全部置 1 的全字段位图，适合实验性
// 全字段请求。
func FullMACBoardMembersQuotesFieldBitmap() [20]byte {
	return MACFieldBitmap(MACPresetDebug)
}

// MACBoardMembersQuotesFieldBitmapFromBits 按传入的位号列表组装 20 字节字段位图。
//
// 负数或超出 0–159 范围的位号会被忽略，不会产生越界写入。
func MACBoardMembersQuotesFieldBitmapFromBits(bits ...int) [20]byte {
	var bitmap [20]byte
	for _, bit := range bits {
		// 位号必须落在 20 字节位图的 0–159 范围内，越界直接跳过。
		if bit < 0 || bit >= len(bitmap)*8 {
			continue
		}
		// 按 `bit/8` 定位字节、`bit%8` 定位字节内的位，按位或完成置位。
		bitmap[bit/8] |= 1 << uint(bit%8)
	}
	return bitmap
}

// DefaultMACSymbolQuotesFieldBitmap 返回个股批量报价接口使用的默认字段位图。
func DefaultMACSymbolQuotesFieldBitmap() [20]byte {
	return DefaultMACBoardMembersQuotesFieldBitmap()
}

// FullMACSymbolQuotesFieldBitmap 返回个股批量报价接口使用的全字段位图。
func FullMACSymbolQuotesFieldBitmap() [20]byte {
	return FullMACBoardMembersQuotesFieldBitmap()
}

// MACSymbolQuotesFieldBitmapFromBits 按传入的位号列表组装个股批量报价通用位图。
func MACSymbolQuotesFieldBitmapFromBits(bits ...int) [20]byte {
	return MACBoardMembersQuotesFieldBitmapFromBits(bits...)
}
