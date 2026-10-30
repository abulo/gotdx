// Command stock_history 演示查询指定历史日期的历史分时图与历史逐笔成交两类数据，并分别打印字段。
//
// 运行示例：`go run ./examples/stock_history`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 查询 2026-03-16 深市 000001 的历史分时图，仅打印前 10 个时点。
	tickItems, err := client.StockHistoryTickChart(20260316, types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range tickItems[:min(10, len(tickItems))] {
		log.Printf("history_tick price=%.2f avg=%.4f vol=%d", item.Price, item.Avg, item.Vol)
	}

	// 查询同一日期深市 000001 的历史逐笔成交，从偏移 0 开始取 20 笔。
	transItems, err := client.StockHistoryTransaction(20260316, types.MarketSZ.Uint8(), "000001", 0, 20)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range transItems {
		log.Printf("history_trans time=%s price=%.2f vol=%d action=%d", item.Time, item.Price, item.Vol, item.BuyOrSell)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
