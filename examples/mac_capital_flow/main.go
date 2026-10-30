// Command mac_capital_flow 演示通过 MACCapitalFlow 查询个股资金流向，以深市 `000001` 为例打印今日与五日主力、散户净流入。
//
// 运行示例：`go run ./examples/mac_capital_flow`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	reply, err := client.MACCapitalFlow(types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("mac_capital_flow query=%s ext=%s today_main_net_in=%.2f today_retail_net_in=%.2f five_day_main_net_in=%.2f",
		reply.QueryInfo, reply.Ext, reply.TodayMainNetIn, reply.TodayRetailNetIn, reply.FiveDayMainNetIn)
}
