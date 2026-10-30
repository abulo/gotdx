package types

import (
	"fmt"
	"regexp"
	"strings"
)

// DetectMarket 从带市场后缀的证券代码中识别所属市场，并返回市场枚举与去掉后缀的纯代码。
//
// 输入须为 `代码.两位后缀` 形式（如 `000001.SZ`、`600000.SH`、`00700.HK`、`AAPL.US`）；
// 后缀无法匹配已知交易所，或缺少分隔点时返回格式错误。纯代码请先经 `StockPrefix` 或 `DecodeStockCode` 补全后缀。
func DetectMarket(symbol string) (Market, string, error) {
	// 去空格并统一大写，兼容大小写与首尾空白不一致的输入。
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	prefix := symbol[len(symbol)-3:]
	if prefix[0] != '.' {
		return 0, "", fmt.Errorf("股票代码错误,例如:000001.SZ")
	}
	code := symbol[:len(symbol)-3]
	market := symbol[len(symbol)-2:]
	switch market {
	case MarketSH.String():
		return MarketSH, code, nil
	case MarketSZ.String():
		return MarketSZ, code, nil
	case MarketBJ.String():
		return MarketBJ, code, nil
	case MarketHK.String():
		return MarketHK, code, nil
	case MarketUSA.String():
		return MarketUSA, code, nil
	default:
		return 0, "", fmt.Errorf("股票代码错误,例如:000001.SZ")
	}
}

// DecodeStockCode 同时兼容纯代码与带后缀代码，解析出市场枚举与纯证券代码。
//
// 与 `DetectMarket` 的区别在于本函数会先剥离旧后缀、按代码段规则补全标准后缀，再识别市场；
// 代码段规则无法判定市场时返回格式错误。
func DecodeStockCode(code string) (Market, string, error) {
	code = StockPrefix(CleanCode(code))
	return DetectMarket(code)
}

// StockPrefix 根据纯代码规则为股票、ETF、指数代码补全交易所后缀，返回 `代码.后缀`；无法识别时原样返回纯代码。
//
// 判定自上而下短路匹配，优先级依次为沪市 A 股、深市 A 股、北交所股票、港股、美股、沪市 ETF、深市 ETF、
// 沪市指数、深市指数、北交所指数。
func StockPrefix(code string) string {
	code = CleanCode(code)
	switch {
	case isSHStock(code):
		return fmt.Sprintf("%s.%s", code, MarketSH.String())
	case isSZStock(code):
		return fmt.Sprintf("%s.%s", code, MarketSZ.String())
	case isBJStock(code):
		return fmt.Sprintf("%s.%s", code, MarketBJ.String())
	case isHKStock(code):
		return fmt.Sprintf("%s.%s", code, MarketHK.String())
	case isUSAStock(code):
		return fmt.Sprintf("%s.%s", code, MarketUSA.String())
	case isSHETF(code):
		return fmt.Sprintf("%s.%s", code, MarketSH.String())
	case isSZETF(code):
		return fmt.Sprintf("%s.%s", code, MarketSZ.String())
	case isSHIndex(code):
		return fmt.Sprintf("%s.%s", code, MarketSH.String())
	case isSZIndex(code):
		return fmt.Sprintf("%s.%s", code, MarketSZ.String())
	case isBJIndex(code):
		return fmt.Sprintf("%s.%s", code, MarketBJ.String())
	}
	// 所有规则均未命中时保持原纯代码返回，不强行补后缀。
	return code
}

// IndexPrefix 根据纯代码规则为指数代码补全交易所后缀，仅覆盖沪深北三地指数；无法识别时原样返回。
func IndexPrefix(code string) string {
	code = CleanCode(code)
	switch {
	case isSHIndex(code):
		return fmt.Sprintf("%s.%s", code, MarketSH.String())
	case isSZIndex(code):
		return fmt.Sprintf("%s.%s", code, MarketSZ.String())
	case isBJIndex(code):
		return fmt.Sprintf("%s.%s", code, MarketBJ.String())
	}
	return code
}

// IsStock 判断带后缀的证券代码是否为沪深北 A 股股票，不含港股、美股、ETF 与指数。
func IsStock(symbol string) bool {
	return IsSZStock(symbol) || IsSHStock(symbol) || IsBJStock(symbol)
}

// IsSZStock 判断带后缀的代码是否为深圳 A 股代码；要求长度为九位、后缀为 SZ 且代码段符合深市前缀规则。
func IsSZStock(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	market := symbol[len(symbol)-2:]
	code := symbol[:len(symbol)-3]
	// 长度须为九位（六位代码、点号、两位后缀），先排除格式不符的输入。
	return len(symbol) == 9 && market == MarketSZ.String() && isSZStock(code)
}

// IsSHStock 判断带后缀的代码是否为上海 A 股代码；要求长度为九位、后缀为 SH 且代码段符合沪市前缀规则。
func IsSHStock(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	market := symbol[len(symbol)-2:]
	code := symbol[:len(symbol)-3]
	// 长度须为九位（六位代码、点号、两位后缀），先排除格式不符的输入。
	return len(symbol) == 9 && market == MarketSH.String() && isSHStock(code)
}

// IsBJStock 判断带后缀的代码是否为北交所股票代码；要求长度为九位、后缀为 BJ 且代码段符合北交所前缀规则。
func IsBJStock(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	market := symbol[len(symbol)-2:]
	code := symbol[:len(symbol)-3]
	// 长度须为九位（六位代码、点号、两位后缀），先排除格式不符的输入。
	return len(symbol) == 9 && market == MarketBJ.String() && isBJStock(code)
}

// IsETF 判断带后缀的代码是否为沪深 ETF；要求长度为九位、后缀为 SH 或 SZ，且代码段命中对应市场的 ETF 前缀规则。
func IsETF(symbol string) bool {
	if len(symbol) != 9 {
		return false
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	market := symbol[len(symbol)-2:]
	code := symbol[:len(symbol)-3]

	// ETF 前缀规则不区分大小写，统一转小写后再匹配。
	code = strings.ToLower(code)
	switch {
	case market == MarketSH.String() && isSHETF(code):
		return true
	case market == MarketSZ.String() && isSZETF(code):
		return true
	}
	return false
}

// IsIndex 判断带后缀的代码是否为沪深北指数；要求长度为九位、后缀为 SH、SZ 或 BJ，且代码段命中对应市场的指数前缀规则。
func IsIndex(symbol string) bool {
	if len(symbol) != 9 {
		return false
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	market := symbol[len(symbol)-2:]
	code := symbol[:len(symbol)-3]
	switch {
	case market == MarketSH.String() && isSHIndex(code):
		return true
	case market == MarketSZ.String() && isSZIndex(code):
		return true
	case market == MarketBJ.String() && isBJIndex(code):
		return true
	}
	return false
}

func isSHStock(code string) bool {
	code = CleanCode(code)
	if len(code) != 6 {
		return false
	}
	// 沪市 A 股：6 开头（600、601、603、605 等）。
	return code[:1] == "6"
}

func isSZStock(code string) bool {
	code = CleanCode(code)
	if len(code) != 6 {
		return false
	}
	// 深市 A 股：0 开头（000、001、002、003），或 30 开头的创业板（300、301）。
	return code[:1] == "0" || code[:2] == "30"
}

func isBJStock(code string) bool {
	code = CleanCode(code)
	if len(code) != 6 {
		return false
	}
	// 北交所股票：92 开头。
	return code[:2] == "92"
}

func isHKStock(code string) bool {
	code = CleanCode(code)
	if len(code) != 5 {
		return false
	}
	// 港股代码为五位纯数字。
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isUSAStock(code string) bool {
	code = CleanCode(code)
	// 美股代码为一至五位字母，按正则整体匹配。
	matched, _ := regexp.MatchString(`^[A-Za-z]{1,5}$`, code)
	return matched
}

// CleanCode 剥离证券代码中的市场后缀，返回点号之前的纯代码段，不校验代码合法性。
//
// 空串原样返回；存在点号时只保留第一个点号之前的部分；不含点号时视为纯代码原样返回。
func CleanCode(code string) string {
	if code == "" {
		return ""
	}
	if idx := strings.Index(code, "."); idx != -1 {
		return code[:idx]
	}
	return code
}

func isSHETF(code string) bool {
	if len(code) != 6 {
		return false
	}
	// 沪市 ETF：50/51/52/53/56/58 开头。
	switch code[:2] {
	case "50", "51", "52", "53", "56", "58":
		return true
	}

	return false
}

func isSZETF(code string) bool {
	if len(code) != 6 {
		return false
	}
	// 深市 ETF：15/16 开头。
	return code[:2] == "15" || code[:2] == "16"
}

func isSHIndex(code string) bool {
	if len(code) != 6 {
		return false
	}
	// 沪市指数：000 开头，另有特殊代码 999999。
	return code[:3] == "000" || code == "999999"
}

func isSZIndex(code string) bool {
	if len(code) != 6 {
		return false
	}
	// 深市指数：399 开头。
	return code[:3] == "399"
}

func isBJIndex(code string) bool {
	if len(code) != 6 {
		return false
	}
	// 北交所指数：899 开头。
	return code[:3] == "899"
}
