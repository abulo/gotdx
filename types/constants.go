package types

import (
	"errors"
	"fmt"
	"strings"
)

// Market 标识证券所属的交易市场，底层为 `uint8`，取值与协议市场编号一一对应。
//
// 该类型为无状态值类型，可按值安全传递；零值表示未知市场。
// String 返回两位大写代码用于拼接证券代码，`Name` 返回中文市场名，`Uint8` 返回协议请求使用的数值。
type Market uint8

// `Market*` 市场代码枚举，按顺序依次取值 0 至 4。
const (
	MarketSZ Market = iota
	MarketSH
	MarketBJ
	MarketHK
	MarketUSA
)

// 品种类型枚举，区分股票、指数、板块与 ETF。
const (
	STOCK = iota // 股票
	INDEX        // 指数
	BLOCK        // 板块
	ETF          // ETF
)

// `Market*Name` 各市场对应的两位大写代码字符串。
const (
	MarketSZName string = "SZ"
	MarketSHName string = "SH"
	MarketBJName string = "BJ"
	MarketHKName string = "HK"
	MarketUSName string = "US"
)

// Uint8 返回 `Market` 的底层 `uint8` 数值。
func (obj Market) Uint8() uint8 { return uint8(obj) }

// String 返回 `Market` 对应的两位大写代码字符串；未知市场返回 `unknown`。
func (obj Market) String() string {
	switch obj {
	case MarketSZ:
		return "SZ"
	case MarketSH:
		return "SH"
	case MarketBJ:
		return "BJ"
	case MarketHK:
		return "HK"
	case MarketUSA:
		return "US"
	default:
		return "unknown"
	}
}

// Name 返回 `Market` 对应的中文市场名；未知市场返回“未知”。
func (obj Market) Name() string {
	switch obj {
	case MarketSH:
		return "上海"
	case MarketSZ:
		return "深圳"
	case MarketBJ:
		return "北京"
	case MarketHK:
		return "香港"
	case MarketUSA:
		return "美国"
	default:
		return "未知"
	}
}

// BlockType 表示个股所属的板块分类，由 `Block` 与 `GetStockBlock` 按代码前缀判定得出。
type BlockType uint8

// `BlockType*` 个股板块分类集合，覆盖主板、科创板、创业板、北证、港股、美股等。
const (
	BlockMainBoard BlockType = iota // 主板
	BlockSTAR                       // 科创板
	BlockChiNext                    // 创业板
	BlockBJSE                       // 北证
	BlockHK                         // 港股
	BlockUS                         // 美港
	BlockOther                      // 其他
)

// Block 根据证券代码判定个股所属板块分类。
//
// 沪深北 A 股按代码前缀进一步细分主板、科创板、创业板与北证；港股与美股直接归入对应海外板块。
// 代码无法解析时返回 `BlockOther` 与错误，市场未知时返回 `BlockOther`。
func Block(code string) (BlockType, error) {
	market, symbol, err := DecodeStockCode(code)
	if err != nil {
		return BlockOther, err
	}
	// 板块判定只依赖代码前缀，统一拼接后缀仅为复用 `GetStockBlock` 的入参格式。
	newCode := fmt.Sprintf("%s.%s", symbol, MarketSH.String())
	switch market {
	case MarketSH:
		return GetStockBlock(newCode), nil
	case MarketSZ:
		return GetStockBlock(newCode), nil
	case MarketBJ:
		return GetStockBlock(newCode), nil
	case MarketHK:
		return BlockHK, nil
	case MarketUSA:
		return BlockUS, nil
	default:
		return BlockOther, nil
	}
}

// GetStockBlock 根据六位股票代码判定所属 A 股板块。
//
// 入参为六位数字代码（如 `600000`、`688001`、`300750`、`920001`、`830799`）；
// 非六位代码归入 `BlockOther`，其余按前缀判定科创板、创业板、北证、主板，不匹配者归入其他板块。
func GetStockBlock(code string) BlockType {
	// 统一去除常见交易所后缀，仅保留代码段参与前缀判定。
	code = strings.TrimSuffix(code, ".SH")
	code = strings.TrimSuffix(code, ".SZ")
	code = strings.TrimSuffix(code, ".BJ")

	// 长度必须为六位，否则前缀判定不可靠，归入其他板块。
	if len(code) != 6 {
		return BlockOther
	}

	prefix3 := code[:3]
	prefix1 := code[:1]
	switch {
	// 科创板：688 开头。
	case prefix3 == "688":
		return BlockSTAR
	// 创业板：300、301 开头。
	case prefix1 == "3":
		return BlockChiNext
	case prefix1 == "9":
		return BlockBJSE
	case prefix1 == "0" || prefix1 == "6":
		return BlockMainBoard
	default:
		return BlockOther
	}
}

// `ExMarket*` 扩展行情协议的大类品种与市场代码，编号与服务端品种类型一一对应。
const (
	ExMarketStock        uint8 = 1  // 股票
	ExMarketHK           uint8 = 2  // 香港
	ExMarketFutures      uint8 = 3  // 期货
	ExMarketFX           uint8 = 4  // 汇率
	ExMarketIndex        uint8 = 5  // 指数
	ExMarketValuation    uint8 = 6  // 估值
	ExMarketMoney        uint8 = 7  // 资金
	ExMarketFund         uint8 = 8  // 基金
	ExMarketMonetaryFund uint8 = 9  // 货币基金
	ExMarketIndicator    uint8 = 10 // 指标
	ExMarketMirror       uint8 = 11 // 镜像
	ExMarketOption       uint8 = 12 // 期权
	ExMarketUS           uint8 = 13 // 美国
	ExMarketDE           uint8 = 14 // 德国
	ExMarketSG           uint8 = 15 // 新加坡
)

// `ExCategory*` 扩展市场细分类别代码，作为扩展行情请求的品类参数使用。
// 代码与交易所含义见常量名称缩写（郑州、大连、上海、中金所、COMEX、NYMEX、CBOT 等），个别代码语义待确认。
const (
	ExCategoryTempStock           uint8 = 1
	ExCategoryZZFuturesOption     uint8 = 4
	ExCategoryDLFuturesOption     uint8 = 5
	ExCategorySHFuturesOption     uint8 = 6
	ExCategoryCFFEXOption         uint8 = 7
	ExCategorySHStockOption       uint8 = 8
	ExCategorySZStockOption       uint8 = 9
	ExCategoryBasicFX             uint8 = 10
	ExCategoryCrossFX             uint8 = 11
	ExCategoryIntlIndex           uint8 = 12
	ExCategoryCOMEXFutures        uint8 = 16
	ExCategoryNYMEXFutures        uint8 = 17
	ExCategoryCBOTFutures         uint8 = 18
	ExCategoryHKFinancialFutures  uint8 = 23
	ExCategoryHKFinancialOptions  uint8 = 24
	ExCategoryHKStockFutures      uint8 = 25
	ExCategoryHKStockOptions      uint8 = 26
	ExCategoryHKIndex             uint8 = 27
	ExCategoryZZFutures           uint8 = 28
	ExCategoryDLFutures           uint8 = 29
	ExCategorySHFutures           uint8 = 30
	ExCategoryHKMainBoard         uint8 = 31
	ExCategoryOpenEndFund         uint8 = 33
	ExCategoryMonetaryFund        uint8 = 34
	ExCategoryMacroIndicator      uint8 = 38
	ExCategoryFuturesIndex        uint8 = 42
	ExCategoryBToH                uint8 = 43
	ExCategoryNEEQ                uint8 = 44
	ExCategorySHGold              uint8 = 46
	ExCategoryCFFEXFutures        uint8 = 47
	ExCategoryHKGEM               uint8 = 48
	ExCategoryHKFund              uint8 = 49
	ExCategoryTreasuryValuation   uint8 = 54
	ExCategorySunshinePrivateFund uint8 = 56
	ExCategoryBrokerCollective    uint8 = 57
	ExCategoryBrokerMonetary      uint8 = 58
	ExCategoryMainFuturesContract uint8 = 60
	ExCategoryCSIIndex            uint8 = 62
	ExCategoryGZArbitrageFutures  uint8 = 65
	ExCategoryGZFutures           uint8 = 66
	ExCategoryGZOptions           uint8 = 67
	ExCategoryRiskControlIndex    uint8 = 68
	ExCategoryHuazhengIndex       uint8 = 69
	ExCategoryExtendedSectorIndex uint8 = 70
	ExCategoryHKStock             uint8 = 71
	ExCategoryGEStock             uint8 = 73
	ExCategoryUSStock             uint8 = 74
	ExCategorySGStock             uint8 = 78
	ExCategoryMoneyMarket         uint8 = 91
	ExCategoryFundValuation       uint8 = 93
	ExCategoryHKDarkPool          uint8 = 98
	ExCategoryCodeMirror          uint8 = 100
	ExCategorySZSEIndex           uint8 = 102
)

// `Default*` 各行情接口未显式指定条数或大小时使用的默认值。
const (
	DefaultSecurityListCount = 1600
	DefaultTickChartCount    = 0xba00
	DefaultAuctionCount      = 500
	DefaultUnusualCount      = 600
	DefaultTopBoardSize      = 20
	DefaultQuotesListCount   = 80
	DefaultExListCount       = 2000
	DefaultExQuotesCount     = 100
	DefaultDownloadSize      = 0x7530
)

// `Adjust*` K 线复权方式：0 不复权，1 前复权，2 后复权。
const (
	AdjustNone uint16 = 0
	AdjustQFQ  uint16 = 1
	AdjustHFQ  uint16 = 2
)

// `Category*` 主行情证券列表分类代码，用于按市场拉取证券列表。
const (
	CategorySH  uint8 = 0  // 上证A
	CategorySZ  uint8 = 2  // 深证A
	CategoryA   uint8 = 6  // A股
	CategoryB   uint8 = 7  // B股
	CategoryKCB uint8 = 8  // 科创板
	CategoryBJ  uint8 = 12 // 北证A
	CategoryCYB uint8 = 14 // 创业板
)

// `Filter*` 行情列表筛选条件位图，可按位组合同时排除新股、科创板、ST 股、创业板、港股通、北交所、核准制与注册制标的。
const (
	FilterNew          uint16 = 1
	FilterKC           uint16 = 2
	FilterST           uint16 = 4
	FilterCY           uint16 = 8
	FilterHKConnect    uint16 = 16
	FilterBJ           uint16 = 32
	FilterApproval     uint16 = 64
	FilterRegistration uint16 = 128
)

// `Sort*` 行情列表排序字段代码，指定行情榜单按哪一个字段排序。
// 字段语义按常量名称前缀区分：昨收、买卖价、成交量、成交额、涨跌、内外盘、委比、锁定、换手率、
// 量比、主力净流入、涨跌停时间等。
const (
	SortCode              uint16 = 0x0
	SortName              uint16 = 0x1
	SortPreClose          uint16 = 0x2
	SortOpen              uint16 = 0x3
	SortHigh              uint16 = 0x4
	SortLow               uint16 = 0x5
	SortPrice             uint16 = 0x6
	SortBid               uint16 = 0x7
	SortAsk               uint16 = 0x8
	SortVolume            uint16 = 0x9
	SortTotalAmount       uint16 = 0x0a
	SortLastVolume        uint16 = 0x0b
	SortChange            uint16 = 0x0c
	SortIndustry          uint16 = 0x0d
	SortChangePct         uint16 = 0x0e
	SortAmplitudePct      uint16 = 0x0f
	SortAvg               uint16 = 0x10
	SortPEDynamic         uint16 = 0x11
	SortEntrustRatio      uint16 = 0x12
	SortInsideVolume      uint16 = 0x13
	SortOutsideVolume     uint16 = 0x14
	SortInOutRatio        uint16 = 0x15
	SortBidVolume         uint16 = 0x17
	SortAskVolume         uint16 = 0x18
	SortLockedRatio       uint16 = 0x1b
	SortLockedAmount      uint16 = 0x1c
	SortOpenAmount        uint16 = 0x1d
	SortOpenTurnoverPct   uint16 = 0x1e
	SortVolRatio          uint16 = 0x23
	SortTurnoverRate      uint16 = 0x24
	SortFloatShares       uint16 = 0x25
	SortFloatMarketCap    uint16 = 0x26
	SortTotalMarketCapAB  uint16 = 0x27
	SortUnmatchedVolume   uint16 = 0x2a
	SortStrengthPct       uint16 = 0x2d
	SortSpeedPct          uint16 = 0x2e
	SortActivity          uint16 = 0x2f
	SortShortTurnoverPct  uint16 = 0xcc
	SortVolSpeedPct       uint16 = 0xd0
	SortChange5DPct       uint16 = 0xd1
	SortChange10DPct      uint16 = 0xd2
	SortMainNetAmount     uint16 = 0xd4
	SortMainNetRatio      uint16 = 0xd7
	SortAuctionLimitBuy   uint16 = 0x102
	SortAuctionLimitSell  uint16 = 0x103
	SortLimitUpTime       uint16 = 0x108
	SortLimitDownTime     uint16 = 0x109
	SortOpenSnatchPct     uint16 = 0x10a
	SortAmount2M          uint16 = 0x10c
	SortOpenPct           uint16 = 0x119
	SortHighPct           uint16 = 0x11a
	SortLowPct            uint16 = 0x11b
	SortAvgChangePct      uint16 = 0x11c
	SortDrawdownPct       uint16 = 0x11e
	SortAttackPct         uint16 = 0x11f
	SortYTDPct            uint16 = 0x121
	SortConsecutiveUpDays uint16 = 0x138
	SortChange3DPct       uint16 = 0x139
	SortDividendYieldRate uint16 = 0x13a
	SortChange20DPct      uint16 = 0x172
	SortChange60DPct      uint16 = 0x173
	SortPrevChangePct     uint16 = 0x17e
	SortMTDPct            uint16 = 0x19c
	SortChange1YPct       uint16 = 0x19d
	SortPETTM             uint16 = 0x3ee
	SortPEStatic          uint16 = 0x3ef
)

// `SortOrder*` 行情列表排序方向：0 默认，1 降序，2 升序。
const (
	SortOrderNone uint16 = 0
	SortOrderDesc uint16 = 1
	SortOrderAsc  uint16 = 2
)

// `BlockFile*` 通达信服务端板块数据文件名，依次对应默认板块、地区板块、风格板块与概念板块。
const (
	BlockFileDefault = "block.dat"
	BlockFileZS      = "block_zs.dat"
	BlockFileFG      = "block_fg.dat"
	BlockFileGN      = "block_gn.dat"
)

// `BoardType*` 板块类型代码，用于板块列表与 MAC 板块接口的板块分类参数。
// 已知取值：0 行业、1 行业二级、3 概念、4 风格、5 地区、6 未知、7 其他行业、255 全部；其余细分编码语义待确认。
const (
	BoardTypeHY      uint16 = 0
	BoardTypeHY2     uint16 = 1
	BoardTypeGN      uint16 = 3
	BoardTypeFG      uint16 = 4
	BoardTypeDQ      uint16 = 5
	BoardTypeUnknown uint16 = 6
	BoardTypeHYOther uint16 = 7
	BoardTypeOther11 uint16 = 8
	BoardTypeOther12 uint16 = 9
	BoardTypeAll     uint16 = 255
)

// `ExBoardType*` 扩展市场（港股、美股）板块类型代码，分别对应全部、概念与行业板块。
const (
	ExBoardTypeHKAll uint16 = 0
	ExBoardTypeHKGN  uint16 = 1
	ExBoardTypeHKHY  uint16 = 2
	ExBoardTypeUSAll uint16 = 3
	ExBoardTypeUSGN  uint16 = 4
	ExBoardTypeUSHY  uint16 = 5
)

// KLINE_TYPE_* K 线周期类型代码，作为 K 线请求的周期参数使用。
const (
	KLINE_TYPE_5MIN      = 0  // 5 分钟K 线
	KLINE_TYPE_15MIN     = 1  // 15 分钟K 线
	KLINE_TYPE_30MIN     = 2  // 30 分钟K 线
	KLINE_TYPE_1HOUR     = 3  // 1 小时K 线
	KLINE_TYPE_DAILY     = 4  // 日K 线
	KLINE_TYPE_WEEKLY    = 5  // 周K 线
	KLINE_TYPE_MONTHLY   = 6  // 月K 线
	KLINE_TYPE_EXHQ_1MIN = 7  //  1 分钟
	KLINE_TYPE_1MIN      = 8  // 1 分钟K 线
	KLINE_TYPE_RI_K      = 9  // 日K 线
	KLINE_TYPE_3MONTH    = 10 // 季K 线
	KLINE_TYPE_YEARLY    = 11 // 年K 线
	KLINE_TYPE_SECONDS   = 13 // 多秒K 线
)

// ErrBadData 是服务端返回数据超过协议可承受大小（8M 以上）时抛出的错误。
var (
	ErrBadData = errors.New("more than 8M data")
)
