package extractor

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type regressionCase struct {
	document string
	present  []string
	absent   []string
}

var regressionCases = []regressionCase{
	{"chpl_45396", []string{"stopnia ≥2."}, []string{"stopnia >2."}},
	{"chpl_40223", []string{"Powrót do ≥ 50"}, []string{"Powrót do > 50"}},
	{"chpl_33668", []string{"≥98%", "≤0,1% (A/A)"}, []string{">98%", "<0,1% (A/A)"}},
	{"chpl_17527", []string{"zakrzepowo-zatorowej", "α₁ₐ"}, []string{"zakrzepowo zatorowej", "α1a"}},
	{"chpl_49588", []string{"(Cₘₐₓ)"}, []string{"(C max)", "(Cmax)"}},
	{"chpl_43963", []string{"25 x 10⁹/l"}, []string{"10 9/l"}},
	{"chpl_9745", []string{"obrzęk<br>naczynioruchowy", "zespół<br>Lyella (TEN)"}, []string{"|  |  | wykwity"}},
	{"chpl_45108", []string{"|  | Ból głowy G1-4: 4,1%"}, []string{"\nBól głowy G1-4"}},
	{"chpl_41431", []string{"cytrynianu.\n\n"}, []string{"cytrynianu. Pełny wykaz"}},
	{"chpl_5255", []string{"zwłaszcza na początku leczenia"}, []string{"zwłaszcza na\n\npoczątku"}},
	{"chpl_42852", nil, []string{"V002"}},
	{"chpl_23337", []string{"Wydłużenie<br>odstępu QTc"}, nil},
	{"chpl_10186", []string{"<u>Bakterie beztlenowe</u>"}, nil},
	{"chpl_44618", []string{"Gatunki zwykle wrażliwe"}, []string{"<u>Gatunki zwykle wrażliwe</u>"}},
	{"chpl_37364", []string{"ANOVAₗₙ"}, []string{"ANOVAln"}},
	{"chpl_13440", []string{"Akatyzja⁶", "Parkinsonizm⁶", "4.4)¹²", "odstawienia⁷, ¹²", "wywiadzie¹¹"}, []string{"Akatyzja6", "4.4) 12", "⁷, 12"}},
	{"chpl_21682", []string{"anoreksja¹"}, []string{"anoreksja 1", "anoreksja ¹"}},
	{"chpl_33124", []string{"senność²,", "omdlenia⁴,¹⁶,", "drgawek¹,", "pozapiramidowe¹, ²¹"}, []string{"senność 2", "omdlenia 4", "drgawek 1"}},
	{"chpl_10824", []string{"znakowanego ¹⁴C"}, []string{"znakowanego¹⁴", "14C"}},
	{"chpl_33124_lists", []string{"- w leczeniu schizofrenii;", "\n  - leczenie umiarkowanych do ciężkich epizodów maniakalnych"}, []string{"σ "}},
	{"chpl_45243", []string{"\n- Badanie fazy II dotyczące zespołów mielodysplastycznych:\n  - Wszystkie powiązane z leczeniem"}, []string{"o Wszystkie"}},
	{"chpl_11902", []string{"\n- Jednoczesne podawanie gemfibrozylu"}, []string{"  - Jednoczesne"}},
	{"chpl_11981", []string{"\n  - zakażenia górnych dróg oddechowych;", "\n  - zapalenie osierdzia;"}, nil},
	{"chpl_45211", []string{"\n\nc Ropień", "\n\nj Dysgeuzja"}, []string{"oddechowych c Ropień"}},
	{"chpl_23897", []string{"(10 mg weterynaryjnego produktu leczniczego/kg masy ciała × Masa ciała (kg) leczonego cielęcia) / (Średnie spożycie wody (l) przez cielę w godzinach rannych lub wieczornych) = … mg weterynaryjnego produktu leczniczego na l wody do picia"}, []string{"| x |"}},
	{"chpl_17601", []string{"Szybkość ciągłej infuzji podskórnej (ml/h) = (1,25 ng/kg/min × 60 kg × 0,00006) / (1 mg/ml) = 0,005 ml/h"}, []string{"| = |"}},
	{"chpl_30082", []string{"| Zaburzenia oka |  |\n| Rzadko | Zaburzenia wzroku |\n| Nieznana | Ostra jaskra z zamkniętym kątem<br>przesączania |\n| Zaburzenia serca |  |", "| Zaburzenia czynności nerek i dróg moczowych |  |\n| Nieznana | Zaburzenia czynności nerek, ostra<br>niewydolność nerek |", "| Nieznana | Kurcze mięśni |\n| Zaburzenia układu rozrodczego i piersi |  |\n| Często | Impotencja |", "| Zaburzenia psychiczne |  |\n| Rzadko | Depresja, zaburzenia snu |"}, []string{"Zaburzenia oka<br>Rzadko", "Nieznana Kurcze mięśni", "Często Impotencja"}},
	{"chpl_40981", []string{"| niezbyt często: | zapalenie skóry, nadwrażliwość na światło |\n| rzadko: | toksyczna nekroliza naskórka (zespół Lyella) |\n| bardzo rzadko: | zespół Stevensa-Johnsona |", "| niezbyt często: | ostre i przewlekłe zapalenie trzustki, porażenna niedrożność jelita, refluks żołądkowo-przełykowy, zaburzenia opróżniania żołądka |"}, []string{"\nbardzo rzadko: zespół", "|  | przełykowy"}},
	{"chpl_19300", []string{"| <u>Zaburzenia naczyniowe</u> |  |", "| Niezbyt często: | niedociśnienie ortostatyczne, niedociśnienie tętnicze.<br>Niedociśnienie ortostatyczne lub niedociśnienie tętnicze jest rzadko ciężkie. |", "| <u>Zaburzenia układu oddechowego, klatki piersiowej i śródpiersia:</u> |  |"}, []string{"| Zaburzenia naczyniowe |"}},
	{"chpl_45023", []string{"| Liczba pacjentów | 7332 |  | 7339 |  |  |  |", "| 839 (11,4) | 4,1 | 851 (11,6) | 4,2 | 0,98<br>(0,89–1,08) | <0,001 |"}, []string{"| 7339 |  | 0,98"}},
	{"chpl_21053", []string{"| Osmolarność teoretyczna [mOsm/l] | 1290 |"}, []string{"| Osmolarność teoretyczna [mOsm/l] |  |"}},
	{"chpl_50352", []string{"| Zaburzenia<br>żołądka i jelit | Bardzo często | Biegunka | 614 (53%) | 65 (6%) | 2 (< 1%) |", "|  | Często | Zapalenie błony<br>śluzowej jamy<br>ustnej | 96 (8%) |"}, []string{"|  | Bardzo często | Biegunka | 614"}},
	{"chpl_41154", []string{"| Zaburzenia<br>metabolizmu<br>i odżywiania | Częstość nieznana | Kwasica metaboliczna z dużą luką anionową³ |"}, []string{"| Zaburzenia<br>metabolizmu<br>i odżywiania | Bardzo rzadko"}},
	{"chpl_29661", []string{"| Grupa placebo | 10% | 10% |", "| Grupa donepezyl 5 mg | 18%* | 18%* |"}, []string{"| Grupa placebo | Populacja"}},
	{"chpl_20496", []string{"| Kategoria pacjentów | Catalet C - dawki w ml |  |  |  |", "| Odstępy pomiędzy<br>dawkami przy przejściu do<br>wyższego stężenia | 7 dni | 14 dni | 14 dni |  |"}, []string{"|  |  |  |  |  |  |"}},
	{"chpl_49670", []string{"| Fenytoina | AUC 21% ↓<br>Brak konieczności modyfikacji<br>dawki. | Brak<br>ᵃ AUC 20% ↑<br>ᵃ Cₘₐₓ 20% ↑ |", "ᵃ na podstawie badania"}, []string{"<br>a AUC"}},
	{"chpl_25036", []string{"AlAT (GPT) i γ-GT co", "<u>W przypadku stosowania produktu leczniczego Ursofalk zawiesina doustna do rozpuszczania cholesterolowych kamieni żółciowych:</u>"}, []string{"i g-GT", "</u> <u>"}},
	{"chpl_10463", []string{"| Zaburzenia układu<br>nerwowego | często | senność, zawroty głowy, ból głowy |", "|  | niezbyt często | udar naczyniowy mózgu, niedoczulica, omdlenia,<br>drżenie, apatia |"}, []string{"często<br>niezbyt często"}},
	{"chpl_38704", []string{"|  | Brak | Małe | Umiarkowane | Ciężkie |  |  |", "| Ciężkie,<br>leczenie<br>hemodializą<br>(n=6) | Ciężkie,<br>leczenie<br>CAPD<br>(n=4) |", "| CLR<br>(ml/min) (SD) | 383,2<br>(101,8) | 197,9<br>(78,1) | 135,6<br>(31,6) | 40,3<br>(10,1) | Nie dotyczy | Nie dotyczy |", "| CLT/F<br>(ml/min) (SD) | 588,1<br>(153,7) |", "AUC(0-T)<br>(ng·h /ml)<br>(CV)"}, []string{"Ciężkie, Ciężkie, leczenie", "\nCLR 383,2", "§•"}},
	{"chpl_42052", []string{"Osłabienie mięśni◊◊, kurcze", "ból kości◊, ból i"}, []string{"◊◊ ,", "◊ ,"}},
	{"chpl_2214_table", []string{"| Zaburzenia<br>naczyniowe |  | Krwawienie |"}, []string{"Stevensa- | Polekowe"}},
	{"chpl_2214", []string{"- kumaryny\n\n  U pacjentów przyjmujących warfarynę", "- cyklosporyna\n\n  U pacjentów otrzymujących"}, []string{"kumaryny U pacjentów"}},
	{"chpl_23010", []string{"\n6.2 Niezgodności farmaceutyczne", "\n6.3 Okres ważności"}, []string{"| 6.2 |", "| 6.3 |"}},
	{"chpl_20937", []string{"\n5.2 Okres waznosci", "\n5. DANE FARMACEUTYCZNE:", "\n5.1 Glowne niezgodnosci farmaceutyczne"}, []string{"n CO 5.2", "n Trueperella", "CO 5.2", "o ^ - 11"}},
	{"chpl_17604_nabla", []string{"∇ (ml/h) = D (ng/kg mc./min)", "Szybkości infuzji ∇ (ml/h)"}, []string{"Ñ"}},
	{"chpl_17604", []string{"Szybkość ciągłej infuzji (ml/h) = (Dawka (ng/kg mc./min) × Masa ciała (kg) × 0,00006*) / (Stężenie fiolki Remodulin (mg/ml))", "Szybkość ciągłej infuzji podskórnej (ml/h) = (1,25 ng/kg/min × 60 kg × 0,00006) / (1 mg/ml) = 0,005 ml/h"}, []string{"Szybkość x x", "\n= ("}},
	{"chpl_17604_iv", []string{"Ilość produktu Remodulin (ml) = (0,018 mg/ml) / (1 mg/ml) × 50 ml = 0,9 ml", "= (5 ng/kg mc./min × 60 kg × 0,00006) / (1 ml/h) = 0,018 mg/ml (18 000 ng/ml)", "Krok 2 Ilość produktu Remodulin (ml) = (Stężenie rozcieńczonego produktu Remodulin do podania dożylnego (mg/ml)) / (Stężenie fiolki Remodulin (mg/ml)) × Łączna objętość rozcieńczonego roztworu produktu Remodulin w zbiorniku (ml)"}, []string{"| 0,018 mg/ml |", "<u>5 ng/kg"}},
	{"chpl_29608", []string{"| Najlepsza odpowiedź | Wszystkie dawki (n=147)<br>400 mg (n=73)<br>600 mg (n=74)<br>n (%) |"}, []string{"|  | Wszystkie dawki"}},
	{"chpl_26854", []string{"aldosteron\n\n  Pacjenci z silnie pobudzonym", "\n  - z ciężkim nadciśnieniem tętniczym"}, []string{"aldosteron Pacjenci"}},
	{"chpl_32636", []string{"| Klirens<br>kreatyniny<br>(mL/min) | Dawkowanie | Objętość wlewu¹ i<br>czas² |", "| < 30 | 2 mg (2 mL koncentratu do<br>sporządzania<br>roztworu do infuzji) | 500 mL / 1 godzinę |"}, []string{"Klirens Dawkowanie"}},
	{"chpl_45243_anc", []string{"| Jeśli ANC | Zalecane postępowanie |", "| Spadnie do < 1 x 10⁹/l na co najmniej 7 dni, lub<br>Spadnie do < 1 x 10⁹/l z towarzyszącą gorączką<br>(temperatura ciała ≥ 38,5°C) lub<br>Spadnie do < 0,5 x 10⁹/l | Przerwanie leczenia lenalidomidem i wykonywanie<br>pełnej morfologii krwi nie rzadziej niż raz w<br>tygodniu |", "lub spadnie do<br>< 0,5 x 10⁹/l | Przerwanie leczenia lenalidomidem |", "| Powróci do ≥ 1 x 10⁹/l | Wznowienie leczenia lenalidomidem w dawce na<br>następnym niższym poziomie (poziom dawki -1) |"}, []string{"\nSpadnie do < 0,5 x 10⁹/l\n", "| < 0,5 x 10⁹/l |"}},
}

func TestRealDocumentRegressions(t *testing.T) {
	for _, testCase := range regressionCases {
		t.Run(testCase.document, func(t *testing.T) {
			markdown := documentMarkdownOf(t, filepath.Join("testdata", "regression", testCase.document+".pdf"))
			for _, fragment := range testCase.present {
				if !strings.Contains(markdown, fragment) {
					t.Errorf("missing %q", fragment)
				}
			}
			for _, fragment := range testCase.absent {
				if strings.Contains(markdown, fragment) {
					t.Errorf("unexpected %q", fragment)
				}
			}
		})
	}
}

var (
	letterSpacedRegexp  = regexp.MustCompile(`(?:^|[ \n])(?:\pL ){5,}\pL(?:[ \n.,:]|$)`)
	tableSeparatorRegex = regexp.MustCompile(`^\|( --- \|)+$`)
)

func TestMarkdownInvariantsOnAllFixtures(t *testing.T) {
	var paths []string
	for _, pattern := range []string{"testdata/chpl/*.pdf", "testdata/regression/*.pdf", "testdata/fonts/*.pdf"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, matches...)
	}
	if len(paths) < 40 {
		t.Fatalf("expected at least 40 fixture documents, found %d", len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			markdown := documentMarkdownOf(t, path)
			for _, garbage := range []string{"ÿý", "�", "♣", lineEndHyphen, "˚", "º", "<u></u>", "⁐"} {
				if strings.Contains(markdown, garbage) {
					t.Errorf("contains %q", garbage)
				}
			}
			if strings.Count(markdown, "<u>") != strings.Count(markdown, "</u>") {
				t.Errorf("unbalanced underline tags: %d opening, %d closing", strings.Count(markdown, "<u>"), strings.Count(markdown, "</u>"))
			}
			if match := letterSpacedRegexp.FindString(markdown); match != "" {
				t.Errorf("letter-spaced run %q", match)
			}
			lines := strings.Split(markdown, "\n")
			for index, line := range lines {
				if !tableSeparatorRegex.MatchString(line) || index == 0 {
					continue
				}
				columns := strings.Count(line, "---")
				for row := index - 1; row < len(lines) && strings.HasPrefix(lines[row], "|"); row++ {
					if cells := strings.Count(lines[row], " | ") + 1; cells != columns {
						t.Errorf("row %q has %d cells, table has %d columns", lines[row][:mdMinInt(len(lines[row]), 80)], cells, columns)
					}
				}
			}
		})
	}
}

func mdMinInt(first, second int) int {
	if first < second {
		return first
	}
	return second
}
