// Command stock_list 演示按市场、偏移与数量分页查询证券清单，并打印代码、名称、昨收价与成交量单位。
//
// 运行示例：`go run ./examples/stock_list`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 从偏移 0 开始取深市 20 条证券列表。
	items, err := client.StockList(types.MarketSZ.Uint8(), 0, 20)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("code=%s name=%s pre_close=%.2f vol_unit=%d", item.Code, item.Name, item.PreClose, item.VolUnit)
	}
}
