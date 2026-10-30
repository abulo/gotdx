package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// MACFileList 查询远端文件的大小、状态标志与校验哈希，供下载前确认文件版本。
//
// 使用时先由 NewMACFileList 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACFileList struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACFileListRequest
	reply      *MACFileListReply
}

// MACFileListRequest 保存文件元信息查询的偏移与文件名参数。
type MACFileListRequest struct {
	Offset   uint32
	Filename [70]byte
	Reserved [30]byte
}

// MACFileListReply 保存文件元信息响应的偏移、大小、状态标志与哈希。
type MACFileListReply struct {
	Offset uint32
	Size   uint32
	Flag   int8
	Hash   string
}

// NewMACFileList 创建文件元信息查询对象；请求参数为空时使用内置默认参数。
func NewMACFileList(req *MACFileListRequest) *MACFileList {
	obj := &MACFileList{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACFileListRequest),
		reply:      new(MACFileListReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACFILELIST
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACFileList) applyRequest(req *MACFileListRequest) {
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACFileList) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACFILELIST, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足 41 字节时返回错误。
func (obj *MACFileList) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	// 元信息头固定 41 字节，不足视为非法响应，避免后续切片越界。
	if len(data) < 41 {
		return fmt.Errorf("invalid mac file list response length: %d", len(data))
	}
	obj.reply.Offset = binary.LittleEndian.Uint32(data[:4])
	obj.reply.Size = binary.LittleEndian.Uint32(data[4:8])
	obj.reply.Flag = int8(data[8])
	// 哈希为定长 32 字节，尾部以空字符填充，解码后去除填充符。
	obj.reply.Hash = strings.TrimRight(string(data[9:41]), "\x00")
	return nil
}

// Response 返回解码完成的文件元信息响应；未调用 ParseResponse 前内容为零值。
func (obj *MACFileList) Response() *MACFileListReply {
	return obj.reply
}

// MACFileDownload 按分片下载远端文件内容，由上层按偏移拼接为完整文件。
//
// 使用时先由 NewMACFileDownload 构造对象，再依次调用 BuildRequest、ParseResponse，最后读取响应结果。
// 该对象面向单次请求使用，内部未做并发同步，不应在多个并发协程间共享；零值不可直接使用。
type MACFileDownload struct {
	reqHeader  *ReqHeader
	respHeader *RespHeader
	request    *MACFileDownloadRequest
	reply      *MACFileDownloadReply
}

// MACFileDownloadRequest 保存文件下载的分片序号、字节偏移、分片大小与文件名参数。
type MACFileDownloadRequest struct {
	Index    uint32
	Offset   uint32
	Size     uint32
	Filename [70]byte
	Reserved [30]byte
}

// MACFileDownloadReply 保存下载响应的分片序号、本分片大小与文件原始字节。
type MACFileDownloadReply struct {
	Index uint32
	Size  uint32
	Data  []byte
}

// NewMACFileDownload 创建文件分片下载对象；请求参数为空时使用内置默认分片参数。
func NewMACFileDownload(req *MACFileDownloadRequest) *MACFileDownload {
	obj := &MACFileDownload{
		reqHeader:  new(ReqHeader),
		respHeader: new(RespHeader),
		request:    new(MACFileDownloadRequest),
		reply:      new(MACFileDownloadReply),
	}
	obj.reqHeader.Zip = 0x0c
	obj.reqHeader.SeqID = seqID()
	obj.reqHeader.PacketType = 0x01
	obj.reqHeader.Method = KMSG_MACFILEDOWNLOAD
	// 默认从第 1 个分片开始，单次请求 30000 字节；调用方传入非零值时再覆盖。
	obj.request.Index = 1
	obj.request.Size = 30000
	if req != nil {
		obj.applyRequest(req)
	}
	return obj
}

func (obj *MACFileDownload) applyRequest(req *MACFileDownloadRequest) {
	if req.Index == 0 {
		req.Index = 1
	}
	if req.Size == 0 {
		req.Size = 30000
	}
	obj.request = req
}

// BuildRequest 编码请求参数并返回完整协议报文；序列化失败时返回错误。
func (obj *MACFileDownload) BuildRequest() ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.LittleEndian, obj.request); err != nil {
		return nil, err
	}
	return buildExRequest(KMSG_MACFILEDOWNLOAD, payload.Bytes())
}

// ParseResponse 解码服务端返回的字节流并填充响应；报文长度不足 8 字节时返回错误。
func (obj *MACFileDownload) ParseResponse(header *RespHeader, data []byte) error {
	obj.respHeader = header
	if len(data) < 8 {
		return fmt.Errorf("invalid mac file download response length: %d", len(data))
	}
	obj.reply.Index = binary.LittleEndian.Uint32(data[:4])
	obj.reply.Size = binary.LittleEndian.Uint32(data[4:8])
	// 复制头部之后的全部负载，避免响应切片引用被复用的底层报文缓冲。
	obj.reply.Data = append([]byte(nil), data[8:]...)
	return nil
}

// Response 返回解码完成的文件分片响应；未调用 ParseResponse 前内容为零值。
func (obj *MACFileDownload) Response() *MACFileDownloadReply {
	return obj.reply
}
