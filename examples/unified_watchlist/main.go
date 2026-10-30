// Command unified_watchlist 演示统一客户端在同一会话内同时查询 A 股行情、A 股 K 线、港股与美股报价以及扩展市场分时采样。
//
// 运行示例：`go run ./examples/unified_watchlist`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewUnifiedClient()
	defer client.Disconnect()

	// 查询深市 000001、沪市 600000、深市 300750 三只 A 股的实时行情。
	stockItems, err := client.StockQuotesDetail(
		[]uint8{types.MarketSZ.Uint8(), types.MarketSH.Uint8(), types.MarketSZ.Uint8()},
		[]string{"000001", "600000", "300750"},
	)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range stockItems {
		log.Printf("stock code=%s time=%s price=%.2f open=%.2f high=%.2f low=%.2f vol=%d turnover=%.2f%%",
			item.Code, item.ServerTime, item.Price, item.Open, item.High, item.Low, item.Vol, item.Turnover)
	}

	// 查询深市 000001 最近 5 根日 K 线，不做复权处理。
	bars, err := client.StockKLine(types.KLINE_TYPE_DAILY, types.MarketSZ.Uint8(), "000001", 0, 5, 1, types.AdjustNone)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range bars {
		log.Printf("stock_kline time=%s close=%.2f vol=%.0f turnover=%.2f%%", item.DateTime, item.Close, item.Vol, item.Turnover)
	}

	// 查询美股 `TSLA` 与港股 `09988` 的扩展市场报价。
	exItems, err := client.ExQuotes(
		[]uint8{types.ExCategoryUSStock, types.ExCategoryHKStock},
		[]string{"TSLA", "09988"},
	)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range exItems {
		log.Printf("ex code=%s date=%s close=%.2f high=%.2f low=%.2f vol=%d",
			item.Code, item.Date, item.Close, item.High, item.Low, item.Vol)
	}

	// 查询美股 `TSLA` 的分时采样价格序列，仅打印前 10 个价格点。
	samples, err := client.ExChartSampling(types.ExCategoryUSStock, "TSLA")
	if err != nil {
		log.Fatalln(err)
	}
	for i, price := range samples[:min(10, len(samples))] {
		log.Printf("ex_sample index=%d price=%.2f", i, price)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
