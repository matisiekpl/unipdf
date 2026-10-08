package extractor

import "testing"

func TestBrokenWordIsJoinedInTheFormUsedElsewhereInDocument(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"syllable break in cell", "| Zmniejsze-<br>nie apetytu |\nZmniejszenie dawki.", "| Zmniejszenie apetytu |\nZmniejszenie dawki."},
		{"syllable break in prose", "Reakcje nadwraż- liwości są rzadkie. Nadwrażliwości nie obserwowano.", "Reakcje nadwrażliwości są rzadkie. Nadwrażliwości nie obserwowano."},
		{"compound keeps hyphen", "| Zaburzenia mięśniowo-<br>szkieletowe |\nZaburzenia mięśniowo-szkieletowe i tkanki łącznej", "| Zaburzenia mięśniowo-szkieletowe |\nZaburzenia mięśniowo-szkieletowe i tkanki łącznej"},
		{"capitalised start", "| Gruczolako-<br>włókniaki |\nGruczolako-włókniaki sutka", "| Gruczolako-włókniaki |\nGruczolako-włókniaki sutka"},
		{"unknown break stays", "| Zabu-<br>rzenia snu |", "| Zabu-<br>rzenia snu |"},
		{"suspended hyphen stays", "wewnątrz- i zewnątrzkomórkowy", "wewnątrz- i zewnątrzkomórkowy"},
		{"range of numbers stays", "| 1-<br>2 tabletki |\n12 tabletki", "| 1-<br>2 tabletki |\n12 tabletki"},
		{"narrow cell break joined", "| Rabdo<br>mioliza |\nRzadko: rabdomioliza.", "| Rabdomioliza |\nRzadko: rabdomioliza."},
		{"line of separate words stays", "| Zaburzenia<br>krwi |\nZaburzenia serca", "| Zaburzenia<br>krwi |\nZaburzenia serca"},
		{"narrow cell break of compound", "| przedsionkowo<br>komorowy |\nblok przedsionkowo-komorowy", "| przedsionkowo-komorowy |\nblok przedsionkowo-komorowy"},
		{"part used as word elsewhere stays", "| nie<br>dobór |\nniedobór nie występuje", "| nie<br>dobór |\nniedobór nie występuje"},
	}
	for _, testCase := range cases {
		if got := mdJoinBrokenWords(testCase.text); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestLineEndHyphenRemovedByLayoutIsResolved(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"compound known elsewhere", "choroby zakrzepowo\u00ad zatorowej. Choroba zakrzepowo-zatorowa", "choroby zakrzepowo-zatorowej. Choroba zakrzepowo-zatorowa"},
		{"syllable break known elsewhere", "Reakcje nadwraż\u00ad liwości. Nadwrażliwość", "Reakcje nadwrażliwości. Nadwrażliwość"},
		{"syllable break with related word", "| Zaburzenia endokrynolo\u00ad<br>giczne |\nbadanie endokrynologiczne", "| Zaburzenia endokrynologiczne |\nbadanie endokrynologiczne"},
		{"unknown compound keeps hyphen", "wytwarzające beta\u00ad laktamazy", "wytwarzające beta-laktamazy"},
		{"capitals keep hyphen", "kryteria (NCI\u00ad CTCAE).", "kryteria (NCI-CTCAE)."},
		{"digits keep hyphen", "cytochromu P\u00ad 450) i etylo-2\u00ad metylomaślan", "cytochromu P-450) i etylo-2-metylomaślan"},
		{"hyphen before paragraph break", "zakrzepowo\u00ad\n\nzatorowej", "zakrzepowo-\n\nzatorowej"},
		{"short syllable fragment", "| leczenia tygodnio\u00ad<br>wych |", "| leczenia tygodniowych |"},
		{"short first syllable", "obrzęki ob\u00ad wodowe", "obrzęki obwodowe"},
		{"greek prefix keeps hyphen", "antybiotyki β\u00ad laktamowe", "antybiotyki β-laktamowe"},
		{"suspended hyphen before conjunction", "w okresie przed\u00ad i pooperacyjnym", "w okresie przed- i pooperacyjnym"},
		{"single letter prefix", "kwasu p\u00ad aminobenzoesowego", "kwasu p-aminobenzoesowego"},
	}
	for _, testCase := range cases {
		if got := mdJoinBrokenWords(testCase.text); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestLineEndHyphenIsRestoredAfterLastGlyphOfItsLine(t *testing.T) {
	marks := []TextMark{scriptMark("o", 508.21, 421.03, 432.07, 11), scriptMark("z", 70.94, 408.43, 419.47, 11)}
	hyphen := scriptMark("-", 513.73, 421.03, 432.07, 11)

	restored := mdRestoreHyphens(marks, []TextMark{hyphen})

	if got := scriptTexts(restored); len(got) != 3 || got[0] != "o" || got[1] != lineEndHyphen || got[2] != "z" {
		t.Errorf("got %q, want the hyphen right after the last glyph of its line", got)
	}
}

func TestChainedLineEndHyphensAndUnderlinedBreaksAreResolved(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"chain of compounds", "gamma\u00ad glutamylo\u00ad transferazy", "gamma-glutamylo-transferazy"},
		{"break inside underline", "<u>układu Renin-Angiotensin\u00ad</u> <u>Aldosteron</u>", "<u>układu Renin-Angiotensin-Aldosteron</u>"},
	}
	for _, testCase := range cases {
		if got := mdJoinBrokenWords(testCase.text); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestLineEndHyphenAfterDigitJoinsKnownCompound(t *testing.T) {
	got := mdJoinBrokenWords("Ara-C, 6- merkaptopuryna. Podawano 6-merkaptopuryna")

	if want := "Ara-C, 6-merkaptopuryna. Podawano 6-merkaptopuryna"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNumberRangeBrokenAtDashIsJoined(t *testing.T) {
	if got := mdJoinBrokenWords("stężenie 2,8– 12 ng/ml i kwas(S)-3- (aminometylo)"); got != "stężenie 2,8–12 ng/ml i kwas(S)-3-(aminometylo)" {
		t.Errorf("got %q", got)
	}
}

func TestLineEndHyphenBeforeSingleLetterIsKept(t *testing.T) {
	if got := mdJoinBrokenWords("Współczynnik ryzyka | wartość­<br>p† |"); got != "Współczynnik ryzyka | wartość-p† |" {
		t.Errorf("got %q", got)
	}
}
