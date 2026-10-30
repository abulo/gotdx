// Command stock_transaction 演示查询证券当日逐笔成交明细，并打印时间、价格、笔数与买卖方向。
//
// 运行示例：`go run ./examples/stock_transaction`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 从偏移 0 开始查询深市 000001 最近 20 笔逐笔成交。
	items, err := client.StockTransaction(types.MarketSZ.Uint8(), "000001", 0, 20)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("time=%s price=%.2f vol=%d num=%d action=%d",
			item.Time, item.Price, item.Vol, item.Num, item.BuyOrSell)
	}
}
