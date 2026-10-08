package extractor

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func syntheticPDF(objects []string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for index, object := range objects {
		offsets[index] = buffer.Len()
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := buffer.Len()
	fmt.Fprintf(&buffer, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buffer, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buffer.Bytes()
}

func syntheticStream(dictionary, content string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dictionary, len(content), content)
}

func syntheticPageText(t *testing.T, pdf []byte) *PageText {
	reader, err := model.NewPdfReader(bytes.NewReader(pdf))
	if err != nil {
		t.Fatal(err)
	}
	page, err := reader.GetPage(1)
	if err != nil {
		t.Fatal(err)
	}
	extractor, err := New(page)
	if err != nil {
		t.Fatal(err)
	}
	pageText, _, _, err := extractor.ExtractPageText()
	if err != nil {
		t.Fatal(err)
	}
	return pageText
}

func TestFormDrawnTwiceIsExtractedAtBothPositions(t *testing.T) {
	pdf := syntheticPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /X0 4 0 R >> >> /Contents 5 0 R >>",
		syntheticStream("/Type /XObject /Subtype /Form /BBox [0 0 595 842] /Resources << /Font << /F1 6 0 R >> >>", "BT /F1 12 Tf 0 0 Td (Hello) Tj ET"),
		syntheticStream("", "q 1 0 0 1 100 700 cm /X0 Do Q q 1 0 0 1 100 500 cm /X0 Do Q"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	})

	var heights []float64
	for _, mark := range syntheticPageText(t, pdf).Marks().Elements() {
		if mark.Text == "H" {
			heights = append(heights, mark.BBox.Lly)
		}
	}

	if len(heights) != 2 || mdAbs(heights[0]-heights[1]) < 100 {
		t.Errorf("form text found at %v, want two copies 200 points apart", heights)
	}
}

func TestPageFontIsResolvedInPageResourcesAfterForm(t *testing.T) {
	pdf := syntheticPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F0 7 0 R >> /XObject << /X0 4 0 R >> >> /Contents 5 0 R >>",
		syntheticStream("/Type /XObject /Subtype /Form /BBox [0 0 595 842] /Resources << /Font << /F0 6 0 R >> >>", "BT /F0 12 Tf 100 700 Td (abc) Tj ET"),
		syntheticStream("", "/X0 Do BT /F0 12 Tf 100 500 Td (abc) Tj ET"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Symbol >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	})

	text := syntheticPageText(t, pdf).Text()

	if !strings.Contains(text, "abc") || !strings.Contains(text, "αβχ") {
		t.Errorf("got %q, want the page text in Helvetica and the form text in Symbol", text)
	}
}
