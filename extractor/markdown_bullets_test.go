package extractor

import (
	"strings"
	"testing"
)

func TestSymbolFontBulletsStartListItems(t *testing.T) {
	page := testPage(
		testMark("Przeciwwskazania:", 70, 700),
		testMark("♣", 70, 686), testMark("w nadwrażliwości;", 85, 686),
		testMark("♣", 70, 672), testMark("w ospie wietrznej.", 85, 672),
	)

	markdown := page.Markdown()

	if !strings.Contains(markdown, "- w nadwrażliwości;\n- w ospie wietrznej.") {
		t.Errorf("symbol bullets not rendered as list items:\n%s", markdown)
	}
}

func TestSymbolBulletInCellBecomesBulletButDashStays(t *testing.T) {
	cell := mdJoinCell([]mdCellMark{
		{x0: 100, x1: 106, y: 700, s: ""}, {x0: 110, x1: 150, y: 700, s: "nudności"},
		{x0: 100, x1: 106, y: 688, s: "−"}, {x0: 110, x1: 130, y: 688, s: "3,22"},
	})

	if cell != "• nudności<br>− 3,22" {
		t.Errorf("got %q", cell)
	}
}
