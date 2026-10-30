// Command mac_board_members_quotes 演示 MACBoardMembersQuotesWithSort 获取板块 `880761` 的成分股报价，
// 按指定排序类型与顺序取前 20 只并打印行情字段。
//
// 运行示例：`go run ./examples/mac_board_members_quotes`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 排序类型 14、排序顺序 1 的具体取值含义由协议常量定义，此处直接透传演示。
	items, err := client.MACBoardMembersQuotesWithSort("880761", 20, 14, 1)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("mac_member_quote sort_type=%d sort_order=%d symbol=%s name=%s close=%.2f pre_close=%.2f turnover=%.2f pe_static=%.2f",
			14, 1, item.Symbol, item.Name, item.Close, item.PreClose, item.TurnoverRate, item.PEStatic)
	}
}
