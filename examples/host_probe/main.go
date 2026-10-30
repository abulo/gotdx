// Command host_probe 演示对内置主站、扩展站、MAC 主站与 MAC 扩展站四类服务器逐一做 TCP 探测，
// 打印可达性与延迟；最后通过自动选最快主机接口选出延迟最低的主站。
//
// 运行示例：`go run ./examples/host_probe`
package main

import (
	"log"
	"time"

	"github.com/bensema/gotdx"
)

func main() {
	// 分别探测四类内置服务器，每组最多打印前 5 个探测结果。
	logProbe("main", gotdx.MainHosts(), 5)
	logProbe("ex", gotdx.ExHosts(), 5)
	logProbe("mac", gotdx.MACHosts(), 5)
	logProbe("mac-ex", gotdx.MACExHosts(), 5)

	// 开启自动选择最快主机，连接时由客户端在候选地址中挑选延迟最低者。
	client := gotdx.New(gotdx.WithAutoSelectFastest(true), gotdx.WithTimeoutSec(6))
	fastest, err := client.FastestHost()
	if err != nil {
		log.Printf("fastest main host unavailable: %v", err)
		return
	}
	log.Printf("fastest main host: %s %s latency=%s", fastest.Name, fastest.Address, fastest.Latency)
}

// logProbe 对一组主机并发探测可达性与延迟，按返回顺序打印前 `limit` 个结果；
// 不可达主机单独打印错误信息，便于观察候选地址质量。
func logProbe(label string, hosts []gotdx.HostInfo, limit int) {
	results := gotdx.ProbeHosts(hosts, time.Second)
	log.Printf("%s hosts=%d", label, len(hosts))

	for i, result := range results {
		if i >= limit {
			break
		}
		if result.Reachable {
			log.Printf("%s #%d %s %s latency=%s", label, i+1, result.Name, result.Address, result.Latency)
			continue
		}
		log.Printf("%s #%d %s %s error=%s", label, i+1, result.Name, result.Address, result.Error)
	}
}
