package proto

import (
	"bytes"
	"encoding/binary"
)

// GetFileMeta 封装远端板块或资料文件元信息（大小与哈希）查询的请求与响应处理流程。
//
// 先按文件名查询文件大小与哈希，供下载前判断文件版本与是否需要更新。
type GetFileMeta struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *GetFileMetaRequest
	reply      *GetFileMetaReply
}

// GetFileMetaRequest 是文件元信息查询的请求参数。
type GetFileMetaRequest struct {
	Filename [40]byte // 目标文件名。
}

// GetFileMetaReply 是文件元信息查询的响应数据。
type GetFileMetaReply struct {
	Size      uint32   // 文件大小（字节）。
	Unknown1  byte     // 未确认字节，语义待确认。
	HashValue [32]byte // 文件哈希值。
	Unknown2  byte     // 未确认字节，语义待确认。
}

// NewGetFileMeta 创建并初始化文件元信息查询请求对象；传入参数为空时使用默认参数。
func NewGetFileMeta(req *GetFileMetaRequest) *GetFileMeta {
	obj := new(GetFileMeta)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(GetFileMetaRequest)
	obj.reply = new(GetFileMetaReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_BLOCKINFOMETA
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *GetFileMeta) applyRequest(req *GetFileMetaRequest) {
	obj.request = req
}

// BuildRequest 将请求头与请求参数按小端序编码为协议字节流；编码失败时返回错误。
func (obj *GetFileMeta) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 42
	obj.reqHeader.PkgLen2 = 42

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, obj.reqHeader)
	err = binary.Write(buf, binary.LittleEndian, obj.request)
	return buf.Bytes(), err
}

// ParseResponse 解析服务端返回的文件大小与哈希并填充响应对象。
// 响应为定长 38 字节：0:4 大小，4 保留字节，5:37 哈希，37 保留字节。
func (obj *GetFileMeta) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 0:4 文件大小，单位字节。
	obj.reply.Size = binary.LittleEndian.Uint32(data[:4])
	// 保留字节，语义待确认。
	obj.reply.Unknown1 = data[4]
	// 5:37 固定 32 字节哈希值，整体拷贝到定长数组。
	copy(obj.reply.HashValue[:], data[5:37])
	// 保留字节，语义待确认。
	obj.reply.Unknown2 = data[37]
	return nil
}

// Response 返回解析完成后的文件元信息响应数据。
func (obj *GetFileMeta) Response() *GetFileMetaReply {
	return obj.reply
}

// DownloadFile 封装按偏移分段下载远端板块或资料文件内容的请求与响应处理流程。
//
// 大文件无法一次性返回，调用方依据元信息得到的大小按偏移与长度分块请求。
type DownloadFile struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *DownloadFileRequest
	reply      *DownloadFileReply
}

// DownloadFileRequest 是文件分段下载的请求参数。
type DownloadFileRequest struct {
	Start    uint32    // 起始偏移（字节）。
	Size     uint32    // 请求下载长度（字节）。
	Filename [300]byte // 目标文件名。
}

// DownloadFileReply 是文件分段下载的响应数据。
type DownloadFileReply struct {
	Size uint32 // 本次返回数据长度（字节）。
	Data []byte // 文件原始字节内容。
}

// NewDownloadFile 创建并初始化文件下载请求对象；传入参数为空时使用默认参数。
func NewDownloadFile(req *DownloadFileRequest) *DownloadFile {
	obj := new(DownloadFile)
	obj.reqHeader = new(ReqHeader)
	obj.respHeader = new(RespHeader)
	obj.request = new(DownloadFileRequest)
	obj.reply = new(DownloadFileReply)

	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x00
	obj.reqHeader.Method = KMSG_BLOCKINFO
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *DownloadFile) applyRequest(req *DownloadFileRequest) {
	obj.request = req
}

// BuildRequest 将请求头与请求参数按小端序编码为协议字节流；编码失败时返回错误。
func (obj *DownloadFile) BuildRequest() ([]byte, error) {
	obj.reqHeader.PkgLen1 = 310
	obj.reqHeader.PkgLen2 = 310

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, obj.reqHeader)
	err = binary.Write(buf, binary.LittleEndian, obj.request)
	return buf.Bytes(), err
}

// ParseResponse 解析服务端返回的文件数据块并填充响应对象。
// 0:4 为本次返回长度，4 起为文件原始字节。
func (obj *DownloadFile) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 0:4 本次数据块长度。
	obj.reply.Size = binary.LittleEndian.Uint32(data[:4])
	// 4 起为原始字节；显式拷贝独立切片，避免复用底层报文数组造成共享。
	obj.reply.Data = append([]byte(nil), data[4:]...)
	return nil
}

// Response 返回解析完成后的文件下载响应数据。
func (obj *DownloadFile) Response() *DownloadFileReply {
	return obj.reply
}
