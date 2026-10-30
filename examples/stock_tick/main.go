// Command stock_tick 演示查询证券当日分时图序列，并打印每个时点的价格、均价与累计成交量。
//
// 运行示例：`go run ./examples/stock_tick`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 查询深市 000001 当日分时数据，从偏移 0 开始取 60 个时点。
	items, err := client.StockTickChart(types.MarketSZ.Uint8(), "000001", 0, 60)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("price=%.2f avg=%.4f vol=%d", item.Price, item.Avg, item.Vol)
	}
}
