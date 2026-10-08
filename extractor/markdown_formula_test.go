package extractor

import (
	"fmt"
	"strings"
	"testing"
)

func TestFractionTableBecomesFormula(t *testing.T) {
	cases := []struct {
		name  string
		cells [][]string
		want  string
	}{
		{
			"veterinary dose",
			[][]string{
				{"10 mg weterynaryjnego<br>produktu<br>leczniczego/kg masy<br>ciała", "x", "Masa ciała (kg)<br>leczonego cielęcia", "= … mg weterynaryjnego produktu<br>leczniczego na l wody do picia"},
				{"Średnie spożycie wody (l) przez cielę w godzinach<br>rannych lub wieczornych", "", "", ""},
			},
			"(10 mg weterynaryjnego produktu leczniczego/kg masy ciała × Masa ciała (kg) leczonego cielęcia) / (Średnie spożycie wody (l) przez cielę w godzinach rannych lub wieczornych) = … mg weterynaryjnego produktu leczniczego na l wody do picia",
		},
		{
			"left-hand side",
			[][]string{
				{"Szybkość<br>ciągłej infuzji<br>podskórnej<br>(ml/h)", "=", "1,25 ng/kg/min", "x", "60 kg", "x", "0,00006", "= 0,005 ml/h"},
				{"", "", "1 mg/ml", "", "", "", "", ""},
			},
			"Szybkość ciągłej infuzji podskórnej (ml/h) = (1,25 ng/kg/min × 60 kg × 0,00006) / (1 mg/ml) = 0,005 ml/h",
		},
		{
			"left-hand side outside the table",
			[][]string{
				{"=", "40 ng/kg mc./min", "X", "65 kg", "×", "0,00006", "= 0,031 ml/h"},
				{"", "5 mg/ml", "", "", "", "", ""},
			},
			"= (40 ng/kg mc./min × 65 kg × 0,00006) / (5 mg/ml) = 0,031 ml/h",
		},
		{
			"empty trailing cells",
			[][]string{
				{"dawka [g/kg]", "x", "masa [kg]", "= g na l", "", ""},
				{"spożycie wody [l]", "", "", "", "", ""},
			},
			"(dawka [g/kg] × masa [kg]) / (spożycie wody [l]) = g na l",
		},
	}
	for _, testCase := range cases {
		table := &mdLineTable{cells: testCase.cells}

		got, ok := table.formula()

		if !ok || got != testCase.want {
			t.Errorf("%s: got %q (%v), want %q", testCase.name, got, ok, testCase.want)
		}
	}
}

func TestTablesThatAreNotFractions(t *testing.T) {
	cases := map[string][][]string{
		"checkmarks": {
			{"Lek A", "x", "x", "= 2"},
			{"Lek B", "x", "", ""},
		},
		"no operator": {
			{"Dawka", "Masa", "= wynik"},
			{"Spożycie", "", ""},
		},
		"no result": {
			{"Dawka", "x", "Masa"},
			{"Spożycie", "", ""},
		},
		"empty denominator": {
			{"Dawka", "x", "Masa", "= wynik"},
			{"", "", "", ""},
		},
		"three rows": {
			{"Dawka", "x", "Masa", "= wynik"},
			{"Spożycie", "", "", ""},
			{"Uwagi", "", "", ""},
		},
		"single row": {
			{"Dawka", "x", "Masa", "= wynik"},
		},
	}
	for name, cells := range cases {
		if got, ok := (&mdLineTable{cells: cells}).formula(); ok {
			t.Errorf("%s: got formula %q", name, got)
		}
	}
	if _, ok := (*mdLineTable)(nil).formula(); ok {
		t.Errorf("nil table reported as formula")
	}
}

func TestRenderBlocksWritesFormulaInsteadOfTable(t *testing.T) {
	table := &mdLineTable{cells: [][]string{{"dawka", "x", "masa", "= wynik"}, {"spożycie", "", "", ""}}}

	got := mdRenderBlocks([]mdBlock{{text: "Ilość oblicza się według wzoru:"}, {table: table}, {text: "Koniec."}})

	if got != "Ilość oblicza się według wzoru:\n\n(dawka × masa) / spożycie = wynik\n\nKoniec.\n" || strings.Contains(got, "|") {
		t.Errorf("got %q", got)
	}
}

func wordsAt(spec string) []mdWord {
	var words []mdWord
	for _, field := range strings.Fields(spec) {
		at := strings.LastIndex(field, "@")
		bounds := strings.Split(field[at+1:], "-")
		var x0, x1 float64
		fmt.Sscan(bounds[0], &x0)
		fmt.Sscan(bounds[1], &x1)
		words = append(words, mdWord{s: field[:at], x0: x0, x1: x1})
	}
	return words
}

func generalFormula() ([][]mdWord, []float64, []Stroke) {
	lines := [][]mdWord{
		wordsAt("w@72-80 następujący@83-140 sposób:@143-180"),
		wordsAt("Dawka@237.8-270.8 (ng/kg@273.5-302.2 Masa@340.9-366.6"),
		wordsAt("Szybkość@141.2-184.6 x@316.8-322.3 x@388.1-393.6 0,00006*@403.5-444.8"),
		wordsAt("mc./min)@250-286 ciała@337-358 (kg)@360-378"),
		wordsAt("ciągłej@131.0-161.5 infuzji@164.3-194.9 =@214.2-220.4"),
		wordsAt("Stężenie@280.3-318.8 fiolki@321.5-345.9 Remodulin@348.7-400.7"),
		wordsAt("(ml/h)@149.2-176.7"),
		wordsAt("(mg/ml)@322.4-358.5"),
		wordsAt("*Współczynnik@72.0-138.0 przeliczeniowy@140.8-206.2 0,00006@208.9-244.7 =@247.4-254.9 60@257.6-268.6 min/h@271.3-296.4"),
	}
	lineYs := []float64{617, 591.84, 585.6, 578.9, 572.16, 565.2, 559.44, 552.48, 539.76}
	strokes := append(ruleSegments(570, 228.48, 311.28, 324.72, 382.56, 456), ruleSegments(571.44, 228.48, 311.28, 324.72, 382.56, 456)...)
	return lines, lineYs, strokes
}

func TestFractionBarBuildsFormulaFromStackedText(t *testing.T) {
	lines, lineYs, strokes := generalFormula()

	start, end, text := mdNextFraction(lines, lineYs, 0, strokes)

	want := "Szybkość ciągłej infuzji (ml/h) = (Dawka (ng/kg mc./min) × Masa ciała (kg) × 0,00006*) / (Stężenie fiolki Remodulin (mg/ml))"
	if text != want || start != 1 || end != 8 {
		t.Errorf("got %q for lines %d..%d, want %q for 1..8", text, start, end, want)
	}
}

func TestFractionBlockSitsBetweenProse(t *testing.T) {
	lines, lineYs, strokes := generalFormula()

	blocks := mdLineBlocks(lines, lineYs, strokes, 500)

	if len(blocks) != 3 || blocks[0].text != "w następujący sposób:" || !strings.HasPrefix(blocks[1].text, "Szybkość ciągłej infuzji (ml/h) = (") || !strings.HasPrefix(blocks[2].text, "*Współczynnik") {
		t.Errorf("got %+v, want prose, formula, prose", blocks)
	}
}

func TestFractionWithOperatorAndResultOnTheRight(t *testing.T) {
	lines := [][]mdWord{
		wordsAt("Ilość@108-129 produktu@132-172 Remodulin@175-223 0,018@298-323 mg/ml@326-352"),
		wordsAt("(ml)@150-170 =@267-273 x@395-401 50@403-414 ml@417-429 =@434-440 0,9@446-460 ml@462-474"),
		wordsAt("1@318-324 mg/ml@326-352"),
	}
	strokes := ruleSegments(152.6, 280.3, 373.2)

	_, _, text := mdNextFraction(lines, []float64{166, 159.6, 146}, 0, strokes)

	if text != "Ilość produktu Remodulin (ml) = (0,018 mg/ml) / (1 mg/ml) × 50 ml = 0,9 ml" {
		t.Errorf("got %q", text)
	}
}

func TestUnderlinedHeadingIsNotFraction(t *testing.T) {
	lines := [][]mdWord{
		wordsAt("Pacjenci@72-120 z@123-128 zaburzeniami@131-195 czynności@198-245 nerek@248-280"),
		wordsAt("U@72-78 pacjentów@81-130 dawka@133-160 wynosi@163-195 5@198-203 mg@206-220"),
	}
	strokes := ruleSegments(698, 72, 280)

	if _, _, text := mdNextFraction(lines, []float64{702, 688}, 0, strokes); text != "" {
		t.Errorf("got formula %q from an underlined heading", text)
	}
}

func TestFractionNeedsEqualsAndOperatorSides(t *testing.T) {
	strokes := ruleSegments(570, 228, 456)
	cases := map[string][][]mdWord{
		"no equals sign": {
			wordsAt("Dawka@240-270 Masa@340-366"),
			wordsAt("Stężenie@280-318"),
		},
		"left side without equals sign": {
			wordsAt("Szybkość@141-184 Dawka@240-270 =@470-476 5@480-486"),
			wordsAt("infuzji@164-194 Stężenie@280-318"),
		},
		"operator without equals sign": {
			wordsAt("Dawka@240-270 x@470-476 Masa@480-506"),
			wordsAt("Stężenie@280-318"),
		},
		"right side starting with a word": {
			wordsAt("Dawka@240-270 =@214-220 Masa@470-500"),
			wordsAt("Stężenie@280-318"),
		},
		"empty denominator": {
			wordsAt("Dawka@240-270 =@214-220"),
			wordsAt("Szybkość@141-184"),
		},
	}
	for name, lines := range cases {
		if _, _, text := mdNextFraction(lines, []float64{578, 563}, 0, strokes); text != "" {
			t.Errorf("%s: got formula %q", name, text)
		}
	}
}

func TestFormulaTableTakesLeftHandSideFromBeside(t *testing.T) {
	table := &mdLineTable{
		xs:    []float64{190, 210, 300, 320, 380, 400, 450, 520},
		ys:    []float64{415, 405, 395},
		cells: [][]string{{"=", "40 ng/kg mc./min", "x", "65 kg", "x", "0,00006", "= 0,031 ml/h"}, {"", "5 mg/ml", "", "", "", "", ""}},
	}
	marks := []TextMark{
		wordMark("Szybkość", 99.4, 413, 424, 11), wordMark("ciągłej", 145.6, 413, 424, 11),
		wordMark("infuzji", 94.5, 401, 412, 11), wordMark("podskórnej", 127.8, 401, 412, 11),
		wordMark("(ml/h)", 124, 388, 399, 11),
		wordMark("Przykład", 72, 440, 451, 11),
	}
	used := make([]bool, len(marks))

	table.attachFormulaSide(marks, used)
	got, _ := table.formula()

	if got != "Szybkość ciągłej infuzji podskórnej (ml/h) = (40 ng/kg mc./min × 65 kg × 0,00006) / (5 mg/ml) = 0,031 ml/h" || used[5] || !used[0] {
		t.Errorf("got %q with used %v", got, used)
	}
}

func TestFractionCellsAreReleasedOnlyNextToEquals(t *testing.T) {
	fraction := &mdLineTable{xs: []float64{280, 550}, ys: []float64{178, 151.7, 138}, cells: [][]string{{"0,018 mg/ml"}, {"1 mg/ml"}}}
	equals := []TextMark{wordMark("=", 267, 154, 165, 11)}
	far := []TextMark{wordMark("=", 267, 300, 311, 11)}
	word := []TextMark{wordMark("Uwaga", 220, 154, 165, 11)}

	if !fraction.fractionCells(equals, map[int]bool{}) || fraction.fractionCells(far, map[int]bool{}) || fraction.fractionCells(equals, map[int]bool{0: true}) || fraction.fractionCells(word, map[int]bool{}) {
		t.Errorf("fraction cells released wrongly")
	}
	if !(&mdLineTable{cells: [][]string{{"= 0,018 mg/ml<br>(18 000 ng/ml)"}}}).fractionCells(nil, nil) {
		t.Errorf("one-cell result not released")
	}
	for _, cells := range [][][]string{{{"Uwaga"}}, {{"a", "b"}, {"c", "d"}}} {
		if (&mdLineTable{xs: []float64{0, 1, 2}, ys: []float64{178, 138}, cells: cells}).fractionCells(equals, map[int]bool{}) {
			t.Errorf("%v released", cells)
		}
	}
}
