package extractor

import (
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func styledWords(segments ...[]mdWord) []mdWord {
	var line []mdWord
	for _, segment := range segments {
		line = append(line, segment...)
	}
	return line
}

func TestBoldAndItalicRunsAreEmphasised(t *testing.T) {
	line := styledWords(
		styledLine(70, "Azotany", italicStyle),
		proseLine(120, "nasilają"),
		styledLine(170, "działanie hipotensyjne", boldStyle),
		proseLine(300, "awanafilu."),
	)

	if got, want := mdRenderStyledLine(line, nil, true), "*Azotany* nasilają **działanie hipotensyjne** awanafilu."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := mdRenderStyledLine(line, nil, false), "Azotany nasilają działanie hipotensyjne awanafilu."; got != want {
		t.Errorf("without emphasis got %q, want %q", got, want)
	}
}

func TestBoldItalicRunGetsBothMarkers(t *testing.T) {
	line := styledLine(70, "Ostrzeżenie specjalne", boldStyle|italicStyle)

	if got, want := mdRenderStyledLine(line, nil, true), "***Ostrzeżenie specjalne***"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEmphasisSitsInsideUnderline(t *testing.T) {
	line := styledLine(70, "Toksyczne działanie na płuca", boldStyle)
	underline := horizontalRule(-2, line[0].x0, line[len(line)-1].x1)

	if got, want := mdRenderStyledLine(line, []Stroke{underline}, true), "<u>**Toksyczne działanie na płuca**</u>"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunsWithStarsOrWithoutLettersStayPlain(t *testing.T) {
	line := styledWords(
		styledLine(70, "Lek*", boldStyle),
		proseLine(110, "oraz"),
		styledLine(150, "(1)", italicStyle),
	)

	if got, want := mdRenderStyledLine(line, nil, true), "Lek* oraz (1)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDocumentMarkdownMarksItalicSubheadingWithEmphasisOption(t *testing.T) {
	italic := model.NewStandard14FontMustCompile(model.HelveticaObliqueName)
	bold := model.NewStandard14FontMustCompile(model.HelveticaBoldName)
	heading := testMark("Azotany", 70, 700)
	heading.Font = italic
	stress := testMark("przeciwwskazane.", 70, 680)
	stress.Font = bold
	page := testPage(heading, testMark("Skojarzenie", 70, 687), stress)

	emphasised := DocumentMarkdownWithOptions([]*PageText{page}, MarkdownOptions{Emphasis: true})
	if !strings.Contains(emphasised, "*Azotany*") || !strings.Contains(emphasised, "**przeciwwskazane.**") {
		t.Errorf("emphasis missing:\n%s", emphasised)
	}
	plain := DocumentMarkdown([]*PageText{page}, false)
	if strings.Contains(plain, "*") {
		t.Errorf("emphasis without the option:\n%s", plain)
	}
}
