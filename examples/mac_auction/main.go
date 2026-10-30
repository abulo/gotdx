// Command mac_auction 演示通过 MACAuction 获取 MAC 竞价数据，以深市 `000001` 为例打印竞价时间、价格与匹配/未匹配量。
//
// 运行示例：`go run ./examples/mac_auction`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 市场为深市，标的代码 `000001`，从偏移 0 开始请求 100 条竞价记录。
	items, err := client.MACAuction(types.MarketSZ.Uint8(), "000001", 0, 100)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("mac_auction time=%s price=%.2f matched=%d unmatched=%d flag=%d",
			item.Time, item.Price, item.Matched, item.Unmatched, item.Flag)
	}
}
