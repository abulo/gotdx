// Command stock_kline 演示通过统一接口 StockKLine 查询沪深市场 K 线并逐根打印字段。
//
// 运行示例：`go run ./examples/stock_kline`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 查询深市平安银行（000001）最近 10 根秒级 K 线，不做复权。
	bars, err := client.StockKLine(
		types.KLINE_TYPE_SECONDS,
		// types.KLINE_TYPE_1MIN,
		types.MarketSZ.Uint8(),
		"000001",
		0,
		10,
		1,
		types.AdjustNone,
	)
	if err != nil {
		log.Fatalln(err)
	}

	// 逐根打印 K 线的时间、OHLC、涨跌与成交量字段。
	for _, bar := range bars {
		log.Printf("%s preClose=%.3f open=%.3f high=%.3f low=%.3f close=%.3f rate=%.2f  price=%.2f vol=%.0f amount=%.0f turnover=%.2f%%",
			bar.DateTime, bar.PreClose, bar.Open, bar.High, bar.Low, bar.Close, bar.RiseRate, bar.RisePrice, bar.Vol, bar.Amount, bar.Turnover)
	}
}
