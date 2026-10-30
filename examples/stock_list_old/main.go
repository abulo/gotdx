// Command stock_list_old 演示使用旧版证券列表协议查询深市证券清单，并打印代码、名称、昨收价与成交量单位。
//
// 运行示例：`go run ./examples/stock_list_old`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 从偏移 0 开始查询旧版列表，仅打印前 10 条。
	items, err := client.StockListOld(types.MarketSZ.Uint8(), 0)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items[:exampleutil.Min(10, len(items))] {
		log.Printf("old_list code=%s name=%s pre_close=%.2f vol_unit=%d",
			item.Code, item.Name, item.PreClose, item.VolUnit)
	}
}
