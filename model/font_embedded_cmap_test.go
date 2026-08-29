package model

import (
	"testing"

	"github.com/matisiekpl/unipdf/v3/core"
)

const oneByteIdentityCMap = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo 3 dict dup begin
/Registry (Adobe) def
/Ordering (Identity) def
/Supplement 0 def
end def
/CMapName /OneByteIdentityH def
/CMapType 1 def
1 begincodespacerange
<00> <FF>
endcodespacerange
1 begincidrange
<00> <FF> 0
endcidrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end`

func type0FontWithEncoding(t *testing.T, encoding core.PdfObject) *PdfFont {
	t.Helper()
	descendant := core.MakeDict()
	descendant.Set("Type", core.MakeName("Font"))
	descendant.Set("Subtype", core.MakeName("CIDFontType2"))
	descendant.Set("BaseFont", core.MakeName("AAAAAA+TimesNewRomanPSMT"))
	systemInfo := core.MakeDict()
	systemInfo.Set("Registry", core.MakeString("Adobe"))
	systemInfo.Set("Ordering", core.MakeString("Identity"))
	systemInfo.Set("Supplement", core.MakeInteger(0))
	descendant.Set("CIDSystemInfo", systemInfo)

	font := core.MakeDict()
	font.Set("Type", core.MakeName("Font"))
	font.Set("Subtype", core.MakeName("Type0"))
	font.Set("BaseFont", core.MakeName("AAAAAA+TimesNewRomanPSMT"))
	font.Set("DescendantFonts", core.MakeArray(descendant))
	font.Set("Encoding", encoding)

	loaded, err := NewPdfFontFromPdfObject(font)
	if err != nil {
		t.Fatalf("loading font: %v", err)
	}
	return loaded
}

// A composite font is otherwise assumed to use two-byte codes. An embedded CMap declaring a
// one-byte codespace must override that, or every two characters of the page merge into one.
func TestEmbeddedOneByteCMapSplitsCodesPerByte(t *testing.T) {
	stream, err := core.MakeStream([]byte(oneByteIdentityCMap), nil)
	if err != nil {
		t.Fatal(err)
	}
	font := type0FontWithEncoding(t, stream)

	codes := font.BytesToCharcodes([]byte{0x20, 0x32, 0x2e, 0x38})

	want := []int{0x20, 0x32, 0x2e, 0x38}
	if len(codes) != len(want) {
		t.Fatalf("got %d charcodes %v, want %d", len(codes), codes, len(want))
	}
	for index, code := range codes {
		if int(code) != want[index] {
			t.Fatalf("charcodes = %v, want %v", codes, want)
		}
	}
}

func TestIdentityHStillReadsTwoByteCodes(t *testing.T) {
	font := type0FontWithEncoding(t, core.MakeName("Identity-H"))

	codes := font.BytesToCharcodes([]byte{0x20, 0x32, 0x2e, 0x38})

	want := []int{0x2032, 0x2e38}
	if len(codes) != len(want) {
		t.Fatalf("got %d charcodes %v, want %d", len(codes), codes, len(want))
	}
	for index, code := range codes {
		if int(code) != want[index] {
			t.Fatalf("charcodes = %v, want %v", codes, want)
		}
	}
}
