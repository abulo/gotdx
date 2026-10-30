// Command gotdx-examples 是 `gotdx` 整组示例的快速入口：先查询沪深主行情快照，再查询扩展市场（美股）报价；其余专题能力见 `examples` 下各子目录。
//
// 运行示例：`go run ./examples`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewUnifiedClient()
	defer client.Disconnect()

	// 查询深市 000001 与沪市 600008 两只 A 股的主行情快照。
	reply, err := client.StockQuotesDetail([]uint8{types.MarketSZ.Uint8(), types.MarketSH.Uint8()}, []string{"000001", "600008"})
	if err != nil {
		log.Fatalln(err)
	}

	for _, obj := range reply {
		log.Printf("%+v", obj)
	}

	// 切换到扩展市场分类，查询美股 `TSLA` 的报价。
	exQuotes, err := client.ExQuotes([]uint8{types.ExCategoryUSStock}, []string{"TSLA"})
	if err != nil {
		log.Fatalln(err)
	}

	for _, obj := range exQuotes {
		log.Printf("%+v", obj)
	}
}
