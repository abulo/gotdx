package gotdx

import "github.com/bensema/gotdx/proto"

// GetCompanyCategories 查询指定证券的 F10 公司信息章节目录；`market` 取市场代码，
// `code` 为证券代码，返回的文件名、起始位置与长度供 GetCompanyContent 逐章拉取正文。
func (client *Client) GetCompanyCategories(market uint8, code string) (*proto.GetCompanyCategoryReply, error) {
	obj := proto.NewGetCompanyCategory(&proto.GetCompanyCategoryRequest{
		Market: uint16(market),
		Code:   makeCode6(code),
	})
	return client.execute(obj)
}

// GetCompanyContent 按目录条目拉取指定证券的 F10 章节正文；`filename`、`start`、`length`
// 来自 GetCompanyCategories 返回的目录条目，`market` 取市场代码，`code` 为证券代码。
func (client *Client) GetCompanyContent(market uint8, code string, filename string, start uint32, length uint32) (*proto.GetCompanyContentReply, error) {
	obj := proto.NewGetCompanyContent(&proto.GetCompanyContentRequest{
		Market:   uint16(market),
		Code:     makeCode6(code),
		Filename: makeFixed80(filename),
		Start:    start,
		Length:   length,
	})
	return client.execute(obj)
}

// GetFinanceInfo 查询指定证券的财务概况，包括股本、每股指标、资产负债与利润等字段；
// `market` 取市场代码，`code` 为证券代码。
func (client *Client) GetFinanceInfo(market uint8, code string) (*proto.GetFinanceInfoReply, error) {
	obj := proto.NewGetFinanceInfo(&proto.GetFinanceInfoRequest{
		Market: market,
		Code:   makeCode6(code),
	})
	return client.execute(obj)
}

// GetXDXRInfo 查询指定证券的除权除息历史记录；`market` 取市场代码，`code` 为证券代码。
func (client *Client) GetXDXRInfo(market uint8, code string) (*proto.GetXDXRInfoReply, error) {
	obj := proto.NewGetXDXRInfo(&proto.GetXDXRInfoRequest{
		Market: market,
		Code:   makeCode6(code),
	})
	return client.execute(obj)
}

// GetFileMeta 查询主站文件的元信息，包括文件大小、哈希校验值与未确认字段。
func (client *Client) GetFileMeta(filename string) (*proto.GetFileMetaReply, error) {
	obj := proto.NewGetFileMeta(&proto.GetFileMetaRequest{Filename: makeFixed40(filename)})
	return client.execute(obj)
}

// DownloadFile 从 `start` 偏移下载 `size` 字节的主站文件片段；片段大小为 0 表示文件已取完。
func (client *Client) DownloadFile(filename string, start uint32, size uint32) (*proto.DownloadFileReply, error) {
	obj := proto.NewDownloadFile(&proto.DownloadFileRequest{
		Start:    start,
		Size:     size,
		Filename: makeFixed300(filename),
	})
	return client.execute(obj)
}
