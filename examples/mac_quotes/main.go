// Command mac_quotes 演示通过 MAC 协议接口 `MACQuotes` 查询单只证券的实时报价及其分时图表数据并打印。
//
// 运行示例：`go run ./examples/mac_quotes`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 查询深市证券 000001 的最新报价；如需指定历史日期，可改用 `MACQuotesWithDate`，下一行已注释，仅作对照示例。
	reply, err := client.MACQuotes(types.MarketSZ.Uint8(), "000001")
	// reply, err := client.MACQuotesWithDate(types.MarketSZ.Uint8(), "000001", 20260418)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("mac_quotes code=%s name=%s time=%s close=%.2f pre_close=%.2f turnover=%.2f avg=%.2f chart_points=%d",
		reply.Code, reply.Name, reply.DateTime, reply.Close, reply.PreClose, reply.Turnover, reply.Avg, len(reply.ChartData))

	// 仅打印前 5 个分时图点。
	for _, item := range reply.ChartData[:min(5, len(reply.ChartData))] {
		log.Printf("chart time=%s price=%.2f avg=%.2f vol=%d momentum=%.2f",
			item.Time, item.Price, item.Avg, item.Vol, item.Momentum)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
