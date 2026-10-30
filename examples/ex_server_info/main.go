// Command ex_server_info 演示手动构造扩展行情客户端：先完成登录握手并打印服务器信息，
// 再分别查询当日分时与指定交易日历史分时，以美股 `TSLA` 为例。
//
// 运行示例：`go run ./examples/ex_server_info`
package main

import (
	"log"

	"github.com/bensema/gotdx"
	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	hosts := exampleutil.ExHosts()
	// 首条作为主地址，其余进入候选地址池，拨号超时设置为 6 秒。
	client := gotdx.NewEx(
		gotdx.WithExTCPAddress(hosts[0]),
		gotdx.WithExTCPAddressPool(hosts[1:]...),
		gotdx.WithTimeoutSec(6),
	)
	defer client.Disconnect()

	// 连接扩展行情服务器并完成登录握手，打印应答中的登录时间、服务器名与地址。
	login, err := client.ConnectEx()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("login time=%s server=%s ip=%s", login.DateTime, login.ServerName, login.IP)

	// 查询扩展行情服务器信息，打印名称、版本、延迟与签名等字段。
	info, err := client.GetExServerInfo()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("server_info name=%s version=%s delay=%d now=%s sign2=%s", info.ServerName, info.Version, info.Delay, info.TimeNow, info.ServerSign2)

	// 查询当日分时图，仅打印前 10 个分时点。
	tick, err := client.ExGetTickChart(types.ExCategoryUSStock, "TSLA")
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range tick.List[:min(10, len(tick.List))] {
		log.Printf("ex_tick time=%s price=%.2f avg=%.2f vol=%d", item.Time, item.Price, item.Avg, item.Vol)
	}

	// 查询交易日 `20260330` 的历史分时图，同样仅打印前 10 个分时点。
	historyTick, err := client.ExGetHistoryTickChart(20260330, types.ExCategoryUSStock, "TSLA")
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range historyTick.List[:min(10, len(historyTick.List))] {
		log.Printf("ex_history_tick time=%s price=%.2f avg=%.2f vol=%d", item.Time, item.Price, item.Avg, item.Vol)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
