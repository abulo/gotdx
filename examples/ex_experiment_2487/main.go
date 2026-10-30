// Command ex_experiment_2487 演示试验协议 `0x2487` 返回的单品种行情原始响应，以美股 `TSLA` 为例打印各字段与尾部十六进制片段。
//
// 运行示例：`go run ./examples/ex_experiment_2487`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	// 市场类别为美股，标的代码 `TSLA`；尾部原始字节仅截取前 80 个字符预览。
	reply, err := client.ExExperiment2487(types.ExCategoryUSStock, "TSLA")
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("ex_2487 code=%s active=%d close=%.2f price=%.2f vol=%d cur_vol=%d amount=%.2f tail=%s",
		reply.Code, reply.Active, reply.Close, reply.Price, reply.Vol, reply.CurVol, reply.Amount, exampleutil.PreviewHex(reply.TailHex, 80))
}
