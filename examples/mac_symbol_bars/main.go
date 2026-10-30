// Command mac_symbol_bars 演示通过 MAC 协议接口 `MACSymbolBars` 查询证券 K 线并逐根打印行情字段。
//
// 运行示例：`go run ./examples/mac_symbol_bars`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 查询深市 000001 的日 K 线，从偏移 0 开始取 10 根，不做复权处理。
	items, err := client.MACSymbolBars(types.MarketSZ.Uint8(), "000001", types.KLINE_TYPE_DAILY, 1, 0, 10, types.AdjustNone)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("mac_symbol_bars date=%s open=%.2f high=%.2f low=%.2f close=%.2f vol=%.2f",
			item.DateTime, item.Open, item.High, item.Low, item.Close, item.Vol)
	}
}
