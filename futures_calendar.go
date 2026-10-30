package gotdx

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bensema/gotdx/proto"
	_ "modernc.org/sqlite"
)

// futuresCalendarLocation 是期货交易日历与自然时间转换统一使用的时区。
const futuresCalendarLocation = "Asia/Shanghai"

// resolveMACFuturesDateTimes 使用期货交易日历把 MAC 期货 K 线的夜盘条目修正为自然时间。
//
// 无法识别交易所前缀或未配置日历路径时直接返回，不修改原条目；同一交易日的上一交易日
// 查询结果在本次调用内缓存。夜盘时段中 21 点之后的条目归属上一交易日，其余夜盘秒数归属
// 下一自然日。日历打开或查询失败时返回包装后的错误。
func resolveMACFuturesDateTimes(items []proto.MACSymbolBar, code, calendarPath string) error {
	exchange, ok := futuresExchange(code)
	if !ok || calendarPath == "" {
		return nil
	}

	db, err := sql.Open("sqlite", calendarPath)
	if err != nil {
		return fmt.Errorf("open futures trade calendar: %w", err)
	}
	defer db.Close()

	preTradeDates := make(map[uint32]time.Time)
	for index := range items {
		seconds := items[index].RawSeconds
		if !isMACNightSession(seconds) {
			continue
		}
		preTradeDate, found := preTradeDates[items[index].RawYMD]
		if !found {
			preTradeDate, err = futuresPreTradeDate(db, exchange, items[index].RawYMD)
			if err != nil {
				return err
			}
			preTradeDates[items[index].RawYMD] = preTradeDate
		}

		// 21 点后属于上一交易日当晚夜盘；凌晨时段已跨入下一自然日。
		if seconds >= 21*60*60 {
			items[index].DateTime = combineFuturesDateTime(preTradeDate, seconds)
			continue
		}
		items[index].DateTime = combineFuturesDateTime(preTradeDate.AddDate(0, 0, 1), seconds)
	}
	return nil
}

// futuresExchange 根据期货合约代码前缀映射交易所代码；识别失败时第二个返回值为 `false`。
// 返回的交易所代码沿用 `Tushare` 交易日历中的标识（如 SHFE、DCE、CZCE 等）。
func futuresExchange(code string) (string, bool) {
	prefix := strings.TrimRight(strings.ToUpper(strings.TrimSpace(code)), "0123456789")
	switch prefix {
	case "AL", "AO", "AG", "AU", "BR", "BU", "CU", "FU", "HC", "NI", "PB", "RB", "RU", "SN", "SP", "SS", "WR", "ZN":
		return "SHFE", true
	case "BC", "EC", "LU", "NR", "SC":
		return "INE", true
	case "A", "B", "C", "CS", "EB", "EG", "FB", "I", "J", "JD", "JM", "L", "LH", "M", "P", "PG", "PP", "RR", "V", "Y":
		return "DCE", true
	case "AP", "CF", "CJ", "CY", "FG", "JR", "MA", "OI", "PF", "PK", "PM", "PR", "RM", "RS", "SA", "SF", "SM", "SR", "TA", "UR", "WH", "ZC":
		return "CZCE", true
	case "IC", "IF", "IH", "IM", "T", "TF", "TL", "TS":
		return "CFFEX", true
	default:
		return "", false
	}
}

// isMACNightSession 判断 MAC K 线条目是否落在需要交易日历修正的夜盘时段：
// 当日 21:00 之后或次日 05:00 之前。
func isMACNightSession(seconds uint32) bool {
	return seconds >= 21*60*60 || seconds <= 5*60*60
}

// futuresPreTradeDate 查询指定期货交易日对应的上一交易日，返回上海时区的自然日零点；
// 日历中查不到记录时返回包装后的 SQL 查询错误。
func futuresPreTradeDate(db *sql.DB, exchange string, tradeDate uint32) (time.Time, error) {
	var value string
	// 交易日历中 pretrade_date 即上一自然交易日，直接取该交易日自身的记录。
	if err := db.QueryRow(`
		SELECT pretrade_date
		FROM futures_trade_calendar
		WHERE exchange = ? AND cal_date = ? AND is_open = 1`, exchange, fmt.Sprintf("%08d", tradeDate)).Scan(&value); err != nil {
		return time.Time{}, fmt.Errorf("find %s pretrade_date for %08d: %w", exchange, tradeDate, err)
	}
	return parseCalendarDate(value)
}

// combineFuturesDateTime 把日历自然日与 MAC 条目中的当日秒数组合为带时区的时间，
// 秒数按时、分、秒拆分，不做跨天进位处理。
func combineFuturesDateTime(date time.Time, seconds uint32) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), int(seconds/3600), int(seconds%3600/60), int(seconds%60), 0, date.Location())
}

// parseCalendarDate 把 `SQLite` 日历中的 `YYYYMMDD` 日期字符串解析为上海时区当日零点；
// 长度非法、数值非法或组合出不存在日期时返回错误。
func parseCalendarDate(value string) (time.Time, error) {
	if len(value) != 8 {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	location, err := time.LoadLocation(futuresCalendarLocation)
	if err != nil {
		return time.Time{}, fmt.Errorf("load %s: %w", futuresCalendarLocation, err)
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse calendar year %q: %w", value, err)
	}
	month, err := strconv.Atoi(value[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse calendar month %q: %w", value, err)
	}
	day, err := strconv.Atoi(value[6:])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse calendar day %q: %w", value, err)
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	return date, nil
}
