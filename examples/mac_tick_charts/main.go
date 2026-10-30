// Command mac_tick_charts 演示通过 MAC 协议查询最近多个交易日的分时图及其逐时点价格并打印。
//
// 运行示例：`go run ./examples/mac_tick_charts`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 查询深市 000001 最近 3 个交易日的分时图；日期参数传 0 表示取最新日期。
	reply, err := client.MACTickCharts(types.MarketSZ.Uint8(), "000001", 0, 3)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("mac_tick_charts code=%s name=%s time=%s close=%.2f days=%d",
		reply.Code, reply.Name, reply.DateTime, reply.Close, len(reply.Charts))

	for _, day := range reply.Charts {
		log.Printf("day=%s pre_close=%.2f ticks=%d", day.Date, day.PreClose, len(day.Ticks))
		// 每日分时点较多，仅打印前 3 个时点。
		for _, item := range day.Ticks[:min(3, len(day.Ticks))] {
			log.Printf("tick time=%s price=%.2f avg=%.2f vol=%d",
				item.Time, item.Price, item.Avg, item.Vol)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
