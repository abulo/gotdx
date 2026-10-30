// Command stock_block_raw 演示板块文件的元信息查询、分块下载与整包下载，并分别按扁平列表、按板块分组两种方式解析板块文件。
//
// 运行示例：`go run ./examples/stock_block_raw`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	if _, err := client.Connect(); err != nil {
		log.Fatalln(err)
	}

	// 查询默认板块文件的元信息，取得文件总大小。
	meta, err := client.GetFileMeta(types.BlockFileDefault)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("file_meta name=%s size=%d", types.BlockFileDefault, meta.Size)

	// 演示分块下载：从偏移 0 开始取前 1024 字节。
	chunk, err := client.DownloadFile(types.BlockFileDefault, 0, 1024)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("download_chunk size=%d", chunk.Size)

	// 演示整包下载：按元信息中的总大小一次性取完整个板块文件。
	full, err := client.DownloadFullFile(types.BlockFileDefault, meta.Size)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("download_full size=%d", len(full))

	// 平铺解析概念板块文件（`BlockFileGN`），每条为一个成分股条目，仅打印前 10 条。
	flat, err := client.GetParsedBlockFile(types.BlockFileGN)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range flat[:min(10, len(flat))] {
		log.Printf("flat block=%s type=%d code=%s", item.BlockName, item.BlockType, item.Code)
	}

	// 分组解析风格板块文件（`BlockFileFG`），每条为一个板块及其成分股计数，仅打印前 5 条。
	grouped, err := client.GetGroupedBlockFile(types.BlockFileFG)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range grouped[:min(5, len(grouped))] {
		log.Printf("grouped block=%s type=%d stocks=%d", item.BlockName, item.BlockType, item.StockCount)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
