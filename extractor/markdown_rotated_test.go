package extractor

import (
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func rotatedPageMarkdown(t *testing.T, contents string) string {
	t.Helper()
	resources := model.NewPdfPageResources()
	resources.SetFontByName("Courier", model.NewStandard14FontMustCompile(model.CourierName).ToPdfObject())
	extractor := Extractor{resources: resources, contents: contents, mediaBox: r(0, 0, 600, 800)}
	pageText, _, _, err := extractor.ExtractPageText()
	if err != nil {
		t.Fatalf("extracting page: %v", err)
	}
	return pageText.Markdown()
}

func TestTextRotatedNinetyDegreesReadsForward(t *testing.T) {
	markdown := rotatedPageMarkdown(t, `
		BT /Courier 12 Tf 1 0 0 1 50 700 Tm (Dawkowanie) Tj ET
		BT /Courier 12 Tf 0 1 -1 0 300 100 Tm (Produkt leczniczy bez waznego pozwolenia) Tj ET
	`)

	if !strings.Contains(markdown, "Produkt leczniczy bez waznego pozwolenia") || !strings.Contains(markdown, "Dawkowanie") {
		t.Errorf("rotated stamp rendered as:\n%s", markdown)
	}
}

func TestTextUpsideDownReadsForward(t *testing.T) {
	markdown := rotatedPageMarkdown(t, `
		BT /Courier 12 Tf -1 0 0 -1 400 500 Tm (4 tygodnie) Tj ET
	`)

	if !strings.Contains(markdown, "4 tygodnie") {
		t.Errorf("upside-down text rendered as:\n%s", markdown)
	}
}

func TestTextRotatedTwoHundredSeventyDegreesReadsForward(t *testing.T) {
	markdown := rotatedPageMarkdown(t, `
		BT /Courier 12 Tf 0 -1 1 0 300 700 Tm (Wskaznik odpowiedzi) Tj ET
	`)

	if !strings.Contains(markdown, "Wskaznik odpowiedzi") {
		t.Errorf("axis label rendered as:\n%s", markdown)
	}
}

func TestSeparateRotatedCellsStaySeparate(t *testing.T) {
	resources := model.NewPdfPageResources()
	resources.SetFontByName("Courier", model.NewStandard14FontMustCompile(model.CourierName).ToPdfObject())
	extractor := Extractor{resources: resources, contents: `
		BT /Courier 12 Tf -1 0 0 -1 200 500 Tm (4 tygodnie) Tj ET
		BT /Courier 12 Tf -1 0 0 -1 400 500 Tm (8 tygodni) Tj ET
	`, mediaBox: r(0, 0, 600, 800)}
	pageText, _, _, err := extractor.ExtractPageText()
	if err != nil {
		t.Fatalf("extracting page: %v", err)
	}
	marks := pageText.Marks().Elements()
	mdCollapseRotatedRuns(marks)

	var texts []string
	for _, mark := range marks {
		if strings.TrimSpace(mark.Text) != "" {
			texts = append(texts, mark.Text)
		}
	}
	if len(texts) != 2 {
		t.Errorf("two distant upside-down runs collapsed into %q", texts)
	}
}
