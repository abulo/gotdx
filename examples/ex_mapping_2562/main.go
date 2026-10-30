// Command ex_mapping_2562 演示试验协议 `0x2562` 返回的品种映射记录，以中金所期货类别为例打印前 10 条；多数字段含义语义待确认。
//
// 运行示例：`go run ./examples/ex_mapping_2562`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	// 市场类别为中金所期货，起始偏移 0，请求 10 条映射记录。
	items, err := client.ExMapping2562(uint16(types.ExCategoryCFFEXFutures), 0, 10)
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range items {
		log.Printf("ex_mapping_2562 category=%d name=%s index=%d switch=%d codes=[%.2f %.2f %.2f %d %d]",
			item.Category, item.Name, item.Index, item.Switch, item.Code1, item.Code2, item.Code3, item.Code4, item.Code5)
	}
}
