// Command ex_sampling 演示通过 ExChartSampling 获取分时缩略采样价格序列，以美股 `TSLA` 为例打印前 20 个采样点。
//
// 运行示例：`go run ./examples/ex_sampling`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	prices, err := client.ExChartSampling(types.ExCategoryUSStock, "TSLA")
	if err != nil {
		log.Fatalln(err)
	}

	// 采样序列可能较长，仅打印前 20 个价格点及其序号。
	for i, price := range prices[:min(20, len(prices))] {
		log.Printf("sample=%d price=%.2f", i, price)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
