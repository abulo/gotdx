// Command mac_file_query 演示 MAC 文件两步操作：先查询 `StockInfo.dat` 的元信息（偏移、大小、标志、哈希），
// 再按元信息中的大小循环下载完整文件并打印字节数。
//
// 运行示例：`go run ./examples/mac_file_query`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 第一步：获取文件元信息，作为后续下载大小的依据。
	meta, err := client.MACFileList("StockInfo.dat", 0)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("mac_file_list offset=%d size=%d flag=%d hash=%s", meta.Offset, meta.Size, meta.Flag, meta.Hash)

	// 第二步：按文件序号 1 与元信息大小循环下载完整文件。
	content, err := client.MACDownloadFullFile("StockInfo.dat", 1, meta.Size)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("mac_file_download bytes=%d", len(content))
}
