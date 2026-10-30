// Command stock_all 演示一次性拉取深市、沪市、北交所全部证券代码，按普通股票类型过滤后打印代码与名称并统计数量。
//
// 运行示例：`go run ./examples/stock_all`
package main

import (
	"fmt"
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()
	i := 0
	for _, market := range []struct {
		market types.Market
	}{
		{market: types.MarketSZ},
		{market: types.MarketSH},
		{market: types.MarketBJ},
	} {
		// 依次遍历深市、沪市、北交所三个市场。

		if items, err := client.StockAll(market.market.Uint8()); err == nil {
			for _, item := range items {
				code := fmt.Sprintf("%s.%s", item.Code, market.market.String())
				// 仅保留被判定为普通股票的证券，跳过指数、基金、债券等品种。
				if types.IsStock(code) {
					log.Println(code, item.Name)
					i += 1
				}
			}
		}

	}
	fmt.Println(i)
}
