// Command mac_kline_offset 演示通过 MACKLineOffset 获取 MAC K 线偏移信息，从偏移 0 开始按默认条数请求，
// 打印服务端返回的总数与本次返回条数。
//
// 运行示例：`go run ./examples/mac_kline_offset`
package main

import (
	"log"

	"github.com/bensema/gotdx"
	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 起始偏移为 0，条数取内置默认值 `DefaultMACKLineOffsetCount`。
	reply, err := client.MACKLineOffset(0, gotdx.DefaultMACKLineOffsetCount)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("mac_kline_offset total=%d returned=%d", reply.Total, reply.Returned)
}
