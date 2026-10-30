// Command stock_quotes_list 演示按板块分类、排序方式与数量查询行情排行榜列表，并打印价格、涨跌额、成交额与换手率。
//
// 运行示例：`go run ./examples/stock_quotes_list`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 查询 A 股分类，按总成交额降序排列，从偏移 0 开始取前 20 条。
	items, err := client.StockQuotesList(
		types.CategoryA,
		0,
		20,
		types.SortTotalAmount,
		true,
		0,
	)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("code=%s price=%.2f change=%.2f amount=%.0f rise_speed=%.2f turnover=%.2f%%",
			item.Code, item.Price, item.Price-item.PreClose, item.Amount, item.RiseSpeed, item.Turnover)
	}
}
