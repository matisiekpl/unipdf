package extractor

import (
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func proseLine(x0 float64, text string) []mdWord {
	var words []mdWord
	for _, s := range strings.Fields(text) {
		width := 5 * float64(len([]rune(s)))
		words = append(words, mdWord{s: s, x0: x0, x1: x0 + width})
		x0 += width + 3
	}
	return words
}

func renderProse(right float64, lines ...[]mdWord) string {
	lineYs := make([]float64, len(lines))
	for index := range lines {
		lineYs[index] = 700 - 13*float64(index)
	}
	return mdRenderProse(lines, lineYs, nil, right)
}

func TestShortLineEndsParagraph(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Ciąża"),
		proseLine(70, "Produktu leczniczego nie należy podawać kobietom w ciąży, ponieważ może"),
		proseLine(70, "wywoływać wady wrodzone."),
		proseLine(70, "Sacharoza"),
		proseLine(70, "Sodu benzoesan"),
	)

	if want := "Ciąża\n\nProduktu leczniczego nie należy podawać kobietom w ciąży, ponieważ może wywoływać wady wrodzone.\n\nSacharoza\n\nSodu benzoesan"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLineBeforeOrphanedPrepositionContinues(t *testing.T) {
	got := renderProse(308,
		proseLine(70, "Należy zachować ostrożność u kobiet w ciąży,"),
		proseLine(70, "W razie przedawkowania"),
	)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want one paragraph", got)
	}
}

func TestSingleAsteriskFootnoteIsNotBullet(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Zapalenie ścięgna."),
		proseLine(70, "* W związku ze stosowaniem chinolonów"),
	)

	if want := "Zapalenie ścięgna.\n\n* W związku ze stosowaniem chinolonów"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParagraphCutByPageIsJoined(t *testing.T) {
	previous := mdProseBlock("W badaniu DELIVER zdarzenia", [][]mdWord{proseLine(70, "W badaniu DELIVER zdarzenia")}, 205)
	next := mdProseBlock("DKA zgłaszano u 2 pacjentów.", [][]mdWord{proseLine(70, "DKA zgłaszano u 2 pacjentów.")}, 500)
	heading := mdProseBlock("4.9 Przedawkowanie", [][]mdWord{proseLine(70, "4.9 Przedawkowanie")}, 500)
	finished := mdProseBlock("Koniec zdania.", [][]mdWord{proseLine(70, "Koniec zdania.")}, 140)

	if !previous.continuedBy(next) {
		t.Errorf("paragraph reaching the margin without a period was not continued")
	}
	if previous.continuedBy(heading) || finished.continuedBy(next) {
		t.Errorf("heading or finished sentence was joined across pages")
	}
	footnote := mdProseBlock("* na podstawie badań z udziałem pacjentów z akromegalią", [][]mdWord{proseLine(70, "* na podstawie badań z udziałem pacjentów z akromegalią")}, 205)
	nextFootnote := mdProseBlock("** na podstawie badań z udziałem pacjentów z guzami", [][]mdWord{proseLine(70, "** na podstawie badań z udziałem pacjentów z guzami")}, 500)
	if footnote.continuedBy(nextFootnote) {
		t.Errorf("footnote was joined with the next footnote across pages")
	}
}

func TestNumberedHeadingKeepsItsOwnParagraph(t *testing.T) {
	got := renderProse(420,
		proseLine(70, "4.4 Specjalne ostrzeżenia i środki ostrożności dotyczące stosowania"),
		proseLine(70, "Nadwrażliwość na substancję czynną."),
	)

	if want := "4.4 Specjalne ostrzeżenia i środki ostrożności dotyczące stosowania\n\nNadwrażliwość na substancję czynną."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSectionNumberOnItsOwnLineJoinsTitle(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "4.9"),
		proseLine(70, "Przedawkowanie"),
		proseLine(70, "Przedawkowanie apiksabanu może zwiększać ryzyko krwawienia."),
	)

	if want := "4.9 Przedawkowanie\n\nPrzedawkowanie apiksabanu może zwiększać ryzyko krwawienia."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNumberedListItemContinuesOnLowercaseLine(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "1. Odwracalna neutropenia zwykle pojawia się tydzień lub później po rozpoczęciu leczenia"),
		proseLine(70, "dożylnego lub gdy łączna dawka przekroczy 25 g."),
	)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want one list item", got)
	}
}

func TestTableBorderUnderCaptionIsNotUnderline(t *testing.T) {
	words := proseLine(70, "Tabela 3. Dostosowanie dawki")
	strokes := []Stroke{{X1: 70, Y1: 498, X2: 200, Y2: 498}, {X1: 70, Y1: 498, X2: 70, Y2: 400}, {X1: 200, Y1: 498, X2: 200, Y2: 400}}
	for index := range words {
		words[index].baseline = 500
	}

	if got := mdRenderLine(words, strokes); strings.Contains(got, "<u>") {
		t.Errorf("got %q, want the table border ignored", got)
	}
}

func TestLineStartingWithDigitIsNotJoinedAcrossBlankLine(t *testing.T) {
	if got := mdJoinSentences("ul. Maciejkowicka 30\n\n41-503 Chorzów"); got != "ul. Maciejkowicka 30\n\n41-503 Chorzów" {
		t.Errorf("got %q", got)
	}
}

func TestUnderlineStartingAtCellEdgeIsKept(t *testing.T) {
	words := proseLine(70, "Bakterie tlenowe")
	strokes := []Stroke{{X1: 70, Y1: 498, X2: 152, Y2: 498}, {X1: 70, Y1: 520, X2: 70, Y2: 400}, {X1: 300, Y1: 520, X2: 300, Y2: 400}}
	for index := range words {
		words[index].baseline = 500
	}

	if got := mdRenderLine(words, strokes); got != "<u>Bakterie tlenowe</u>" {
		t.Errorf("got %q, want the underline kept", got)
	}
}

func TestFullyUnderlinedLineEndsParagraph(t *testing.T) {
	heading := proseLine(70, "Specjalne środki ostrożności dotyczące bezpiecznego stosowania u gatunków zwierząt:")
	strokes := []Stroke{{X1: 70, Y1: 698.5, X2: heading[len(heading)-1].x1, Y2: 698.5}}
	for index := range heading {
		heading[index].baseline = 700
	}
	body := proseLine(70, "Po leczeniu należy dokładnie monitorować konie.")
	for index := range body {
		body[index].baseline = 687
	}

	got := mdRenderProse([][]mdWord{heading, body}, []float64{700, 687}, strokes, heading[len(heading)-1].x1)

	if !strings.Contains(got, "</u>\n\nPo leczeniu") {
		t.Errorf("got %q, want the underlined heading on its own", got)
	}
}

func TestRuleBetweenLinesEndsParagraph(t *testing.T) {
	intro := proseLine(70, "Częstość występowania działań niepożądanych jest określona następująco:")
	row := proseLine(75, "Bardzo często (≥1/10)")
	for index := range intro {
		intro[index].baseline, intro[index].y = 700, 704
	}
	for index := range row {
		row[index].baseline, row[index].y = 680, 684
	}
	strokes := []Stroke{{X1: 65, Y1: 692, X2: 470, Y2: 692}}

	got := mdRenderProse([][]mdWord{intro, row}, []float64{704, 684}, strokes, intro[len(intro)-1].x1)

	if !strings.Contains(got, "następująco:\n\nBardzo często") {
		t.Errorf("got %q, want the boxed row separated by the rule", got)
	}
}

func TestParenthesisLineContinuesListItem(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "1. Hipokalcemia i (lub) hipokaliemia mogą być związane z występowaniem hipomagnezemii"),
		proseLine(70, "(patrz punkt 4.4)"),
	)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want one list item", got)
	}
}

func TestSentenceStartingAfterShortLineIsNewParagraph(t *testing.T) {
	got := renderProse(420,
		proseLine(70, "Pacjent może odczuwać kołatanie i arytmię."),
		proseLine(70, "Nie podawać z innymi lekami sympatykomimetycznymi."),
	)

	if !strings.Contains(got, "arytmię.\n\nNie podawać") {
		t.Errorf("got %q, want a new paragraph", got)
	}
}

func TestOutdentedLineAfterListItemStartsParagraph(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Althyxin 25-100 mikrogramów:"),
		proseLine(70, "- leczenie wola obojętnego w nadczynności i niedoczynności tarczycy, w terapii skojarzonej"),
		proseLine(85, "z lekami przeciwtarczycowymi w nadczynności tarczycy wywołanej lekami tyreostatycznymi."),
		proseLine(70, "Althyxin 100/150/200 mikrogramów:"),
	)

	if !strings.Contains(got, "tyreostatycznymi.\n\nAlthyxin 100/150/200") {
		t.Errorf("got %q, want the sub-heading outside the list item", got)
	}
}

func TestDoubleSpacedParagraphStaysTogether(t *testing.T) {
	lines := [][]mdWord{
		proseLine(70, "Do najczęściej spotykanych objawów niepożądanych należą: ortostatyczny spadek ciśnienia"),
		proseLine(70, "tętniczego krwi i zaburzenia rytmu serca. Spadek ciśnienia tętniczego pojawia się zwłaszcza na"),
		proseLine(70, "początku leczenia podczas stosowania większych dawek perazyny. Objawy te występują u ok."),
		proseLine(70, "15 % pacjentów. Leczenie zapaści polega na dożylnym podaniu noradrenaliny we wlewie"),
	}
	right := 538.0
	for _, line := range lines {
		line[len(line)-1].x1 = right
	}

	got := mdRenderProse(lines, []float64{421.5, 400.7, 380.1, 359.3}, nil, right)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want one double-spaced paragraph", got)
	}
}

func TestLargerGapThanLineSpacingStillEndsParagraph(t *testing.T) {
	lines := [][]mdWord{
		proseLine(70, "Pierwszy akapit zawiera zdanie, które kończy się w tej linii tekstu, bardzo długiej"),
		proseLine(70, "i zawija się do następnej linii akapitu, która jest równie długa jak poprzednia"),
		proseLine(70, "a potem kończy się kropką na końcu tej długiej linii tekstu akapitu pierwszego."),
		proseLine(70, "Drugi akapit zaczyna się po większym odstępie pionowym od poprzedniego akapitu"),
	}
	right := 538.0
	for _, line := range lines {
		line[len(line)-1].x1 = right
	}

	got := mdRenderProse(lines, []float64{700, 687, 674, 650}, nil, right)

	if !strings.Contains(got, "pierwszego.\n\nDrugi akapit") {
		t.Errorf("got %q, want a paragraph break at the larger gap", got)
	}
}

func TestJoinSentencesJoinsOnlyLongParagraphContinuations(t *testing.T) {
	long := "U pacjentów z niewydolnością serca obserwowano duszność, ból w klatce piersiowej oraz inne przedmiotowe"
	cases := []struct {
		name, text, want string
	}{
		{"long paragraph continues", long + "\n\ni podmiotowe objawy.", long + " i podmiotowe objawy."},
		{"address label", "Al. Jerozolimskie 181C, 02-222 Warszawa\n\ntel.: + 48 22 49 21 301", "Al. Jerozolimskie 181C, 02-222 Warszawa\n\ntel.: + 48 22 49 21 301"},
		{"phone label after long line", long + "\n\nfaks: + 48 22 49 21 309", long + "\n\nfaks: + 48 22 49 21 309"},
		{"excipient list", "glinu glicynian\n\ndisodu edetynian", "glinu glicynian\n\ndisodu edetynian"},
		{"frequency list", long + "\n\nrzadko (≥1/10 000 do <1/1000)", long + "\n\nrzadko (≥1/10 000 do <1/1000)"},
		{"comma ends list item", long + ",\n\nciężkie zaburzenie czynności wątroby", long + ",\n\nciężkie zaburzenie czynności wątroby"},
		{"capital starts sentence", long + "\n\nNie stosować.", long + "\n\nNie stosować."},
	}
	for _, testCase := range cases {
		if got := mdJoinSentences(testCase.text); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestDashOpeningWrappedLineIsNotBullet(t *testing.T) {
	got := renderProse(310,
		proseLine(70, "toksycznej rozpływnej martwicy naskórka (ang. TEN"),
		proseLine(70, "- Toxic Epidermal Necrolysis) lub wysypki polekowej"),
	)
	got = strings.ReplaceAll(got, "\n", "⏎")

	if strings.Contains(got, "⏎") {
		t.Errorf("got %q, want the dash kept inside the sentence", got)
	}
}

func TestDashAfterIntroductionIsBullet(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Produkt jest przeciwwskazany:"),
		proseLine(70, "- w ciąży,"),
		proseLine(70, "- u dzieci."),
	)

	if want := "Produkt jest przeciwwskazany:\n- w ciąży,\n- u dzieci."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFrequencyLabelStartsNewParagraph(t *testing.T) {
	got := renderProse(260,
		proseLine(70, "Często: zaburzenia oddawania moczu"),
		proseLine(70, "Bardzo rzadko: nietrzymanie moczu"),
	)

	if !strings.Contains(got, "moczu\n\nBardzo rzadko:") {
		t.Errorf("got %q, want the frequency label on its own paragraph", got)
	}
}

func TestUnderlinedSubheadingAfterSentenceStartsParagraph(t *testing.T) {
	sentence := proseLine(70, "Blister z laminatu OPA/ALU/PVC w tekturowym pudełku.")
	heading := proseLine(70, "Wielkość opakowań:")
	for index := range sentence {
		sentence[index].baseline, sentence[index].y = 700, 704
	}
	for index := range heading {
		heading[index].baseline, heading[index].y = 687, 691
	}
	strokes := []Stroke{{X1: 70, Y1: 685.5, X2: heading[len(heading)-1].x1, Y2: 685.5}}
	right := sentence[len(sentence)-1].x1

	got := mdRenderProse([][]mdWord{sentence, heading}, []float64{704, 691}, strokes, right)

	if !strings.Contains(got, "pudełku.\n\n<u>Wielkość opakowań:</u>") {
		t.Errorf("got %q, want the underlined sub-heading on its own", got)
	}
}

func TestIndentedContinuationStaysInListItem(t *testing.T) {
	first := proseLine(70, "- 1 pipetka o pojemności 1,34 ml na psa o masie ciała")
	got := renderProse(first[len(first)-1].x1+5,
		first,
		proseLine(77, "Od 10 do 20 kg, zakres dawki od 6,7 do 13,4 mg na kg masy ciała."),
	)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want the indented continuation kept in the item", got)
	}
}

func TestFontStyleFromName(t *testing.T) {
	cases := map[model.StdFontName]int{
		model.TimesRomanName:       0,
		model.TimesBoldName:        boldStyle,
		model.TimesItalicName:      italicStyle,
		model.TimesBoldItalicName:  boldStyle | italicStyle,
		model.HelveticaObliqueName: italicStyle,
	}
	for name, want := range cases {
		if got := mdFontStyle(model.NewStandard14FontMustCompile(name)); got != want {
			t.Errorf("%s: got %d, want %d", name, got, want)
		}
	}
	if mdFontStyle(nil) != 0 {
		t.Errorf("nil font has a style")
	}
}

func styledLine(x0 float64, text string, style int) []mdWord {
	words := proseLine(x0, text)
	for index := range words {
		words[index].style = style
	}
	return words
}

func TestItalicSubheadingWrappedToMarginStartsOwnParagraph(t *testing.T) {
	heading := styledLine(70, "Kontynuacja leczenia: lenalidomid w skojarzeniu z deksametazonem do wystąpienia progresji", italicStyle)
	body := proseLine(70, "Kontynuowanie podawania lenalidomidu w dawce 25 mg raz na dobę zaleca się do progresji.")
	right := heading[len(heading)-1].x1

	got := mdRenderProse([][]mdWord{heading, body}, []float64{700, 687}, nil, right)

	if !strings.Contains(got, "progresji\n\nKontynuowanie") {
		t.Errorf("got %q, want the italic sub-heading separated", got)
	}
}

func TestWrappedItalicHeadingContinuesInItalic(t *testing.T) {
	first := styledLine(70, "Badanie oceniające bezpieczeństwo sercowo-naczyniowe linagliptyny w porównaniu", italicStyle)
	second := styledLine(70, "Z glimepirydem (CAROLINA)", italicStyle)
	right := first[len(first)-1].x1

	got := mdRenderProse([][]mdWord{first, second}, []float64{700, 687}, nil, right)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want one wrapped italic heading", got)
	}
}

func TestBoldWordsInsidePlainLineDoNotBreak(t *testing.T) {
	first := proseLine(70, "Należy zachować ostrożność u pacjentów z niewydolnością nerek oraz wątroby,")
	first[0].style = boldStyle
	second := proseLine(70, "Szczególnie u osób w podeszłym wieku.")
	right := first[len(first)-1].x1

	got := mdRenderProse([][]mdWord{first, second}, []float64{700, 687}, nil, right)

	if strings.Contains(got, "\n") {
		t.Errorf("got %q, want no break caused by a single bold word", got)
	}
}

func TestOneLineParagraphsSeparatedByGapsStaySeparate(t *testing.T) {
	lines := [][]mdWord{
		proseLine(70, "Każda tabletka powlekana zawiera 25 mg syldenafilu w postaci cytrynianu."),
		proseLine(70, "Actigra Forte, 50 mg"),
		proseLine(70, "Każda tabletka powlekana zawiera 50 mg syldenafilu w postaci cytrynianu."),
		proseLine(70, "Pełny wykaz substancji pomocniczych, patrz punkt 6.1."),
	}
	for _, line := range lines {
		line[len(line)-1].x1 = 520
	}

	got := mdRenderProse(lines, []float64{629.0, 603.7, 591.0, 565.8}, nil, 520)

	if !strings.Contains(got, "cytrynianu.\n\nPełny wykaz") {
		t.Errorf("got %q, want the paragraph gap kept", got)
	}
}

func TestNewSentenceAfterShortLineLeavesListItem(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "- ketokonazol zwiększał ekspozycję."),
		proseLine(77, "W drugim badaniu oceniano wpływ karbamazepiny na pomalidomid."),
	)

	if !strings.Contains(got, "ekspozycję.\n\nW drugim") {
		t.Errorf("got %q, want a new paragraph after the finished item", got)
	}
}

func TestFootnoteMarkerOnItsOwnLineJoinsFootnote(t *testing.T) {
	for _, marker := range []string{"a", "1", "**", "†", "ᵃ"} {
		got := renderProse(500,
			proseLine(70, marker),
			proseLine(80, "Wedle uznania lekarza można dodać czynnik stymulujący."),
		)
		if want := marker + " Wedle uznania lekarza można dodać czynnik stymulujący."; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestShortWordLineIsNotTreatedAsFootnoteMarker(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Ciąża"),
		proseLine(70, "Nie stosować w ciąży."),
	)

	if got != "Ciąża\n\nNie stosować w ciąży." {
		t.Errorf("got %q", got)
	}
}

func TestCapitalisedTitleIsNotJoinedAcrossPages(t *testing.T) {
	cover := mdProseBlock("CHARAKTERYSTYKA PRODUKTU LECZNICZEGO", [][]mdWord{proseLine(150, "CHARAKTERYSTYKA PRODUKTU LECZNICZEGO")}, 330)
	next := mdProseBlock("<u>CHARAKTERYSTYKA PRODUKTU LECZNICZEGO</u>", [][]mdWord{proseLine(150, "CHARAKTERYSTYKA PRODUKTU LECZNICZEGO")}, 500)
	paragraph := mdProseBlock("Lek stosuje się u dorosłych", [][]mdWord{proseLine(70, "Lek stosuje się u dorosłych")}, 205)
	heading := mdProseBlock("<u>Dzieci i młodzież</u>", [][]mdWord{proseLine(70, "Dzieci i młodzież")}, 500)

	if cover.continuedBy(next) || paragraph.continuedBy(heading) {
		t.Errorf("a title or an underlined heading was joined across pages")
	}
}

func TestShortItemTitleIsNotJoinedWithParagraphBelow(t *testing.T) {
	got := renderProse(505,
		markerLine(85, "-", 99, "Pacjenci z silnie pobudzonym układem renina-angiotensyna-aldosteron"),
		proseLine(99, "Pacjenci z silnie pobudzonym układem renina-angiotensyna-aldosteron są zagrożeni"),
		proseLine(99, "ostrym, wyraźnym spadkiem ciśnienia krwi."),
	)

	if !strings.Contains(got, "aldosteron\n\n  Pacjenci z silnie pobudzonym układem renina-angiotensyna-aldosteron są zagrożeni ostrym") {
		t.Errorf("got %q, want the item title kept apart from the paragraph", got)
	}
}

func TestItemParagraphKeepsNestedList(t *testing.T) {
	got := renderProse(505,
		markerLine(85, "-", 99, "Pacjenci z silnie pobudzonym układem"),
		proseLine(99, "Ryzyko wystąpienia niedociśnienia dotyczy zwłaszcza pacjentów:"),
		markerLine(110, "•", 125, "z ciężkim nadciśnieniem tętniczym"),
		markerLine(110, "•", 125, "z marskością wątroby"),
	)

	want := "- Pacjenci z silnie pobudzonym układem\n\n  Ryzyko wystąpienia niedociśnienia dotyczy zwłaszcza pacjentów:\n  - z ciężkim nadciśnieniem tętniczym\n  - z marskością wątroby"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestItemLineWithRoomForOneWordOnlyKeepsSentence(t *testing.T) {
	first := markerLine(70.9, "-", 76.5, "kobiet w wieku rozrodczym, chyba że spełnione zostały wszystkie poniższe warunki")
	got := renderProse(first[len(first)-1].x1+60,
		first,
		proseLine(76.5, "Programu Zapobiegania Ciąży."),
	)

	if !strings.Contains(got, "warunki Programu Zapobiegania Ciąży.") {
		t.Errorf("got %q, want the sentence kept in one item", got)
	}
}

func TestItemLineContinuedMidSentenceStaysJoined(t *testing.T) {
	cases := []struct{ first, second string }{
		{"u pacjentów z ciężkimi zaburzeniami (10-15 punktów w skali", "Child’a – Pugh’a);"},
		{"Ból głowy zgłoszono u 28% pacjentów wobec", "Wielu pacjentów leczonych interferonem."},
		{"Klarytromycyna zwiększała wartości", "AUC i Cmax edoksabanu."},
		{"pacjenci byli randomizowani (95%", "CI: od -2,7 do -0,86)."},
		{"co tydzień w pierwszych tygodniach leczenia", "12 miesięcy później."},
	}
	for _, testCase := range cases {
		got := renderProse(505, markerLine(85, "-", 99, testCase.first), proseLine(99, testCase.second))

		if !strings.Contains(got, testCase.first+" "+testCase.second) {
			t.Errorf("got %q, want the item kept in one sentence", got)
		}
	}
}

func TestItemTitleFollowedBySentenceStartingWithOneLetterWord(t *testing.T) {
	got := renderProse(523, markerLine(70.9, "•", 85.2, "kumaryny"), proseLine(85.2, "U pacjentów przyjmujących warfarynę zgłaszano nasilenie działania."))

	if !strings.Contains(got, "kumaryny\n\n  U pacjentów") {
		t.Errorf("got %q", got)
	}
}

func TestMarginJunkIsDropped(t *testing.T) {
	lines := [][]mdWord{
		append(wordsAt("o@40-45 .’@58-64"), proseLine(86, "5. DANE FARMACEUTYCZNE:")...),
		proseLine(86, "Poniewaz nie wykonywano badan dotyczacych zgodnosci, produktu"),
		append(wordsAt("o@44-48 ^@50-54 -@56-59 11@60-63"), proseLine(86, "leczniczego nie wolno mieszac z innymi produktami")...),
		proseLine(86, "Okres waznosci weterynaryjnego produktu leczniczego"),
		wordsAt("C@32-37 O@37-42"),
	}

	kept, ys := mdDropMarginJunk(lines, []float64{700, 687, 674, 661, 648})

	if len(kept) != 4 || len(ys) != 4 || kept[0][0].s != "5." || kept[2][0].s != "leczniczego" {
		t.Errorf("got %d lines starting %q, want junk stripped", len(kept), kept[0][0].s)
	}
}

func TestHangingNumbersAndMarkersStay(t *testing.T) {
	lines := [][]mdWord{
		append(wordsAt("6.2@70-85"), proseLine(110, "Niezgodności farmaceutyczne")...),
		proseLine(110, "Nie wolno mieszać z innymi produktami leczniczymi"),
		append(wordsAt("•@70-75"), proseLine(110, "pierwszy punkt listy z kilkoma słowami")...),
		append(wordsAt("Uwaga@40-70"), proseLine(110, "tekst obok etykiety na marginesie")...),
		proseLine(110, "Okres ważności produktu leczniczego weterynaryjnego"),
	}

	kept, _ := mdDropMarginJunk(lines, []float64{700, 687, 674, 661, 648})

	if kept[0][0].s != "6.2" || kept[2][0].s != "•" || kept[3][0].s != "Uwaga" {
		t.Errorf("got %q %q %q, want numbers, bullets and words kept", kept[0][0].s, kept[2][0].s, kept[3][0].s)
	}
}

func TestSingleIndentedFootnoteLabelStays(t *testing.T) {
	lines := [][]mdWord{
		proseLine(86, "Działania niepożądane zgłaszane w badaniach klinicznych"),
		append(wordsAt("a@60-65"), proseLine(86, "Dane z badania klinicznego fazy III")...),
		proseLine(86, "Kolejny akapit tekstu ciągłego bez etykiet"),
	}

	kept, _ := mdDropMarginJunk(lines, []float64{700, 687, 674})

	if kept[1][0].s != "a" {
		t.Errorf("got %q, want the lone footnote label kept", kept[1][0].s)
	}
}

func TestAlignedLegendLabelsLeftOfMarginStay(t *testing.T) {
	lines := [][]mdWord{
		append(wordsAt("(1)@60-72"), proseLine(100, "Zgłaszane w badaniach klinicznych")...),
		append(wordsAt("(2)@60-72"), proseLine(100, "Zgłaszane po wprowadzeniu do obrotu")...),
		append(wordsAt("*:@60-66"), proseLine(100, "Częstość obliczona na podstawie danych")...),
		append(wordsAt("†:@60-66"), proseLine(100, "Działanie niepożądane zidentyfikowane")...),
		proseLine(100, "Zwykły akapit tekstu po legendzie przypisów"),
	}

	kept, _ := mdDropMarginJunk(lines, []float64{700, 687, 674, 661, 648})

	for index, label := range []string{"(1)", "(2)", "*:", "†:"} {
		if kept[index][0].s != label {
			t.Errorf("line %d: got %q, want %q kept", index, kept[index][0].s, label)
		}
	}
}

func noisyLines(extra ...[]mdWord) [][]mdWord {
	lines := [][]mdWord{
		append(wordsAt("o@20-25"), proseLine(86, "Poniewaz nie wykonywano badan dotyczacych zgodnosci")...),
		append(wordsAt("C@32-37 O@37-42"), proseLine(86, "produktu leczniczego nie wolno mieszac z innymi")...),
		append(wordsAt("^@19-23 >@24-28"), proseLine(86, "Okres waznosci weterynaryjnego produktu leczniczego")...),
	}
	return append(lines, extra...)
}

func TestFewEdgeTokensDoNotTriggerFilter(t *testing.T) {
	lines := noisyLines()[:2]

	kept, _ := mdDropMarginJunk(lines, []float64{700, 687})

	if kept[0][0].s != "o" || kept[1][0].s != "C" {
		t.Errorf("got %q %q, want two stray tokens left alone", kept[0][0].s, kept[1][0].s)
	}
}

func TestNoisyPageKeepsTokensNearMarginAndSectionNumbers(t *testing.T) {
	lines := noisyLines(
		append(wordsAt("ii@74-78"), proseLine(86, "punkt drugi wyliczenia w dokumencie")...),
		append(wordsAt("6.2@50-62"), proseLine(86, "Niezgodności farmaceutyczne produktu")...),
	)

	kept, _ := mdDropMarginJunk(lines, []float64{700, 687, 674, 661, 648})

	if kept[0][0].s != "Poniewaz" || kept[3][0].s != "ii" || kept[4][0].s != "6.2" {
		t.Errorf("got %q %q %q", kept[0][0].s, kept[3][0].s, kept[4][0].s)
	}
}

func TestFullLineStartingWithNumberContinuesOnLowercaseLine(t *testing.T) {
	first := proseLine(70, "1 HAI.U: odpowiednik miana przeciwciał HAI wynoszącego 1 log₁₀ u kawii domowych po podaniu")
	got := renderProse(first[len(first)-1].x1, first, proseLine(70, "szczepionki."))

	if want := "1 HAI.U: odpowiednik miana przeciwciał HAI wynoszącego 1 log₁₀ u kawii domowych po podaniu szczepionki."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAlignedTwoPartLinesAreNotJoined(t *testing.T) {
	first := append(proseLine(70, "Inaktywowany parwowirus świń"), proseLine(400, "nie mniej niż 2 HAI.U")...)
	second := append(proseLine(70, "Inaktywowany szczep Erysipelothrix, serotyp 2"), proseLine(400, "nie mniej niż 1 ELISA U")...)

	got := renderProse(second[len(second)-1].x1, first, second)

	if want := "Inaktywowany parwowirus świń nie mniej niż 2 HAI.U\n\nInaktywowany szczep Erysipelothrix, serotyp 2 nie mniej niż 1 ELISA U"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHeadingFollowedByLowercaseProductNameKeepsItsParagraph(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "6.4 Specjalne środki ostrożności podczas przechowywania"),
		proseLine(70, "bicaVera stay safe / sleep safe: nie przechowywać w temperaturze poniżej 4°C."),
	)

	if !strings.HasPrefix(got, "6.4 Specjalne środki ostrożności podczas przechowywania\n\nbicaVera") {
		t.Errorf("got %q, want the heading in its own paragraph", got)
	}
}

func TestSectionHeadingWithInlineContentKeepsContinuationApart(t *testing.T) {
	first := proseLine(70, "6.1. Wykaz substancji pomocniczych: laktoza, skrobia ziemniaczana, mieszanina laktozy, powidonu")
	got := renderProse(first[len(first)-1].x1, first, proseLine(70, "i krospowidonu, powidon, magnezu stearynian."))

	if !strings.Contains(got, "powidonu\n\ni krospowidonu") {
		t.Errorf("got %q, want the continuation in its own paragraph", got)
	}
}

func TestIntroLineClosingBlockAfterSentenceIsOwnParagraph(t *testing.T) {
	first := proseLine(70, "Cefalotynę można stosować jako substancję wskaźnikową dla cefalosporyn.")
	got := renderProse(first[len(first)-1].x1, first, proseLine(70, "Zakażenia skóry i tkanek miękkich:"))

	if want := "Cefalotynę można stosować jako substancję wskaźnikową dla cefalosporyn.\n\nZakażenia skóry i tkanek miękkich:"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDashBetweenNumbersAcrossLinesIsRange(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Niewielkie krwawienie: ze zmniejszeniem stężenia hemoglobiny o 30"),
		proseLine(70, "– 50 g/L."),
	)

	if want := "Niewielkie krwawienie: ze zmniejszeniem stężenia hemoglobiny o 30 – 50 g/L."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
