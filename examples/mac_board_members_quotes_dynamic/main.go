// Command mac_board_members_quotes_dynamic 演示按字段位图动态解析板块成分报价：在通用字段预设基础上
// 追加主力净流入占比与涨跌类型字段，按涨幅百分比降序排列并过滤 `ST` 标的，取板块 `880761` 前 10 只。
//
// 运行示例：`go run ./examples/mac_board_members_quotes_dynamic`
package main

import (
	"log"

	"github.com/bensema/gotdx"
	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 字段位图决定返回哪些动态字段；先取通用预设，再追加两个额外字段。
	fieldBitmap := gotdx.MACFieldBitmap(
		gotdx.MACPresetCommon,
		gotdx.MACFieldMainNetRatio,
		gotdx.MACFieldChangeUpType,
	)
	// 板块代码 `880761`，取 10 只成分股，按涨幅百分比降序，过滤 `ST` 标的。
	reply, err := client.MACBoardMembersQuotesDynamicWithFilter(
		"880761",
		10,
		uint16(gotdx.MACSortChangePct),
		uint8(gotdx.MACSortOrderDesc),
		fieldBitmap,
		gotdx.MACFilterST,
	)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("mac_member_quote_dynamic total=%d count=%d fields=%d bitmap=%x",
		reply.Total, reply.Count, len(reply.ActiveFields), reply.FieldBitmap)

	for _, item := range reply.Stocks[:min(5, len(reply.Stocks))] {
		log.Printf("symbol=%s name=%s close=%v pre_close=%v turnover=%v pe_ttm=%v main_net_ratio=%v change_up_type=%v",
			item.Symbol,
			item.Name,
			item.Values["close"],
			item.Values["pre_close"],
			item.Values["turnover"],
			item.Values["pe_ttm"],
			item.Values["main_net_ratio"],
			item.Values["change_up_type"],
		)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
