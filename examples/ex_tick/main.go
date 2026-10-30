// Command ex_tick 演示通过 ExTickChart 获取扩展市场分时图，日期传 0 表示取当日分时，以美股 `TSLA` 为例打印前 20 点。
//
// 运行示例：`go run ./examples/ex_tick`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	// 日期参数为 0 时取当日分时，否则查询指定历史日期的分时。
	items, err := client.ExTickChart(types.ExCategoryUSStock, "TSLA", 0)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items[:min(20, len(items))] {
		log.Printf("time=%s price=%.2f avg=%.2f vol=%d", item.Time, item.Price, item.Avg, item.Vol)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
