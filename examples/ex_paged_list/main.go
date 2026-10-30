// Command ex_paged_list 演示先查询扩展市场标的总数，再按固定页大小循环分页拉取 ExList 列表，
// 直到取空或达到最大页数限制，并汇总实际拉取条数。
//
// 运行示例：`go run ./examples/ex_paged_list`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

const (
	// `pageSize` 为每页请求条数，`maxPages` 为本次演示允许翻页的最大页数。
	pageSize = 1000
	maxPages = 3
)

func main() {
	client := exampleutil.NewExClient()
	defer client.Disconnect()

	// 先取标的总数，用于校验分页拉取的覆盖范围。
	total, err := client.ExCount()
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("ex_total=%d page_size=%d max_pages=%d", total, pageSize, maxPages)

	var fetched int
	for page := 0; page < maxPages; page++ {
		start := uint32(page * pageSize)
		items, err := client.ExList(start, pageSize)
		if err != nil {
			log.Fatalf("page=%d start=%d failed: %v", page, start, err)
		}
		// 服务端返回空页即表示已到列表末尾，提前结束翻页。
		if len(items) == 0 {
			break
		}

		// 记录每页首条与末条，便于核对翻页边界。
		first := items[0]
		last := items[len(items)-1]
		fetched += len(items)
		log.Printf("page=%d fetched=%d first=%s/%s last=%s/%s",
			page, len(items), first.Code, first.Name, last.Code, last.Name)
	}

	log.Printf("paged_fetch_done fetched=%d of total=%d", fetched, total)
}
