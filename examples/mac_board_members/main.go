// Command mac_board_members 演示 MACBoardMembersWithSort 获取板块 `880761` 的成分股列表，
// 按指定排序类型与顺序取前 20 只并打印代码、市场与名称。
//
// 运行示例：`go run ./examples/mac_board_members`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 排序类型 14、排序顺序 1 的具体取值含义由协议常量定义，此处直接透传演示。
	items, err := client.MACBoardMembersWithSort("880761", 20, 14, 1)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("mac_board_member sort_type=%d sort_order=%d symbol=%s market=%d name=%s",
			14, 1, item.Symbol, item.Market, item.Name)
	}
}
