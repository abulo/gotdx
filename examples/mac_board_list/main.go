// Command mac_board_list 演示先查询全部类型板块的总数，再按该总数拉取 MAC 板块列表并逐行打印。
//
// 运行示例：`go run ./examples/mac_board_list`
package main

import (
	"fmt"
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()
	// 板块类型取全部板块。
	BoardType := types.BoardTypeAll
	count, _err := client.MACBoardCount(BoardType)
	if _err != nil {
		log.Fatalln(_err)
	}

	// 以板块总数作为列表请求上限，由接口内部自动分页拉取。
	items, err := client.MACBoardList(BoardType, uint32(count))
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("mac_board code=%s name=%s price=%.2f rise_speed=%.2f symbol=%s/%s",
			item.Code, item.Name, item.Price, item.RiseSpeed, item.SymbolCode, item.SymbolName)
	}
	fmt.Println(count)

}
