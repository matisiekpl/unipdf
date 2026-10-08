package extractor

import "testing"

func TestRepairRuneKeepsLatinExtendedLetters(t *testing.T) {
	for _, r := range []rune{'ꞏ', 'Ꞌ', 'ꜰ'} {
		if repaired, ok := repairRune(r); ok {
			t.Errorf("%U repaired into %q, want it kept", r, repaired)
		}
	}
}

func TestRepairRuneStillSplitsMergedCodes(t *testing.T) {
	if repaired, ok := repairRune(0x4142); !ok || repaired != "AB" {
		t.Errorf("got %q (%v), want AB", repaired, ok)
	}
}

func TestSinologicalDotBecomesMiddleDot(t *testing.T) {
	if got := mdGlyphReplacer.Replace("ngꞏh/ml"); got != "ng·h/ml" {
		t.Errorf("got %q", got)
	}
}
