// Command ex_history 演示通过 ExHistoryTransaction 查询扩展市场指定交易日的历史成交明细，以美股 `TSLA` 为例仅打印前 20 条。
//
// 运行示例：`go run ./examples/ex_history`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	// 日期 `20260330` 为目标交易日，市场类别为美股，标的代码 `TSLA`。
	items, err := client.ExHistoryTransaction(20260330, types.ExCategoryUSStock, "TSLA")
	if err != nil {
		log.Fatalln(err)
	}

	// 成交明细可能较长，仅截取前 20 条打印时间、价格、数量与买卖方向。
	for _, item := range items[:min(20, len(items))] {
		log.Printf("time=%s price=%d vol=%d action=%s", item.Time, item.Price, item.Vol, item.Action)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
