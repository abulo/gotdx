// Command ex_board_list 演示通过 ExBoardList 拉取扩展市场行业板块列表，并逐行打印板块代码、名称与行情字段。
//
// 运行示例：`go run ./examples/ex_board_list`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	// 板块类型取行业板块，从偏移 0 开始请求 20 条记录。
	items, err := client.ExBoardList(types.BoardTypeHY, 0, 20)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("ex_board code=%s name=%s price=%.2f rise_speed=%.2f symbol=%s/%s",
			item.Code, item.Name, item.Price, item.RiseSpeed, item.SymbolCode, item.SymbolName)
	}
}
