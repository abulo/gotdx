package gotdx

import "testing"

// buildBlockData 按板块文件布局构造一份合成二进制报文：384 字节固定文件头、2 字节板块总数，
// 随后是一个槽头（9 字节名称、2 字节成员数、2 字节类型）与两只 7 字节成员代码，
// 成员代码区再按 2800 字节定长对齐；报文仅供本地解析测试使用，不连接真实服务器。
func buildBlockData() []byte {
	data := make([]byte, 384+2+2800)
	data[384] = 1                                   // 板块总数（低字节）
	copy(data[386:395], []byte{'T', 'E', 'S', 'T'}) // 9 字节板块名称槽，尾部以空字节填充
	data[395] = 2                                   // 成员数量（低字节）
	data[397] = 3                                   // 板块类型（低字节）
	copy(data[399:406], []byte{'6', '0', '0', '0', '0', '0'})
	copy(data[406:413], []byte{'0', '0', '0', '0', '0', '1'})
	return data
}

func TestParseBlockFlat(t *testing.T) {
	items, err := ParseBlockFlat(buildBlockData())
	if err != nil {
		t.Fatalf("ParseBlockFlat failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: %d", len(items))
	}
	if items[0].BlockName != "TEST" || items[0].Code != "600000" || items[1].Code != "000001" {
		t.Fatalf("unexpected block items: %+v", items)
	}
}

func TestParseBlockGroups(t *testing.T) {
	items, err := ParseBlockGroups(buildBlockData())
	if err != nil {
		t.Fatalf("ParseBlockGroups failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected group count: %d", len(items))
	}
	if items[0].BlockName != "TEST" || items[0].StockCount != 2 || len(items[0].Codes) != 2 {
		t.Fatalf("unexpected block group: %+v", items[0])
	}
}
