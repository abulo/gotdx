// Command mac_transactions 演示通过 MAC 协议查询证券逐笔成交明细，并打印时间、价格、数量与买卖方向。
//
// 运行示例：`go run ./examples/mac_transactions`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 从偏移 0 开始查询深市 000001 最近 20 笔逐笔成交。
	items, err := client.MACTransactions(types.MarketSZ.Uint8(), "000001", 0, 20)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("mac_transactions time=%s price=%.2f vol=%d trade_count=%d action=%d",
			item.Time, item.Price, item.Vol, item.TradeCount, item.BuyOrSell)
	}
}
