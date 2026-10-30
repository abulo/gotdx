// Command icfqs_raw 演示 ICFQS 资讯行情接口：先取话题列表原始响应，再由 ICFQSFormatTables 转换为表格行，
// 打印第一张表的行数与前 5 行内容。
//
// 运行示例：`go run ./examples/icfqs_raw`
package main

import (
	"context"
	"log"

	"github.com/bensema/gotdx"
)

func main() {
	client := gotdx.NewICFQS()

	// 分类与板块代码均传空串，页码从第 1 页开始；返回未经字段解析的原始响应。
	raw, err := client.ICFQSTopicListRaw(context.Background(), "", "", 1)
	if err != nil {
		log.Fatalln(err)
	}

	// 将原始响应中的结果集转换为按列名对齐的表格行。
	tables := gotdx.ICFQSFormatTables(raw)
	if len(tables) == 0 {
		log.Println("icfqs topic list returned no tables")
		return
	}

	log.Printf("icfqs topic list rows=%d", len(tables[0].Rows))
	// 仅打印第一张表的前 5 行，避免输出过长。
	for i, row := range tables[0].Rows[:min(5, len(tables[0].Rows))] {
		log.Printf("#%d %+v", i+1, row)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
