package extractor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/matisiekpl/unipdf/v3/model"
)

type expectedDocument struct {
	Document string          `json:"document"`
	Tables   []expectedTable `json:"tables"`
}

type expectedTable struct {
	Name string     `json:"name"`
	Rows [][]string `json:"rows"`
}

var markdownSeparatorRow = regexp.MustCompile(`^\|( --- \|)+$`)

func documentMarkdownOf(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer file.Close()
	reader, err := model.NewPdfReader(file)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	count, err := reader.GetNumPages()
	if err != nil {
		t.Fatalf("counting pages of %s: %v", path, err)
	}
	var pages []*PageText
	for number := 1; number <= count; number++ {
		page, err := reader.GetPage(number)
		if err != nil {
			t.Fatalf("page %d of %s: %v", number, path, err)
		}
		extractor, err := New(page)
		if err != nil {
			t.Fatalf("extractor for page %d of %s: %v", number, path, err)
		}
		pageText, _, _, err := extractor.ExtractPageText()
		if err != nil {
			t.Fatalf("extracting page %d of %s: %v", number, path, err)
		}
		pages = append(pages, pageText)
	}
	return DocumentMarkdown(pages, true)
}

func markdownTables(markdown string) [][][]string {
	var tables [][][]string
	var current [][]string
	for _, line := range strings.Split(markdown, "\n") {
		if !strings.HasPrefix(line, "|") {
			if current != nil {
				tables = append(tables, current)
				current = nil
			}
			continue
		}
		if markdownSeparatorRow.MatchString(line) {
			continue
		}
		current = append(current, strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "| "), " |"), " | "))
	}
	if current != nil {
		tables = append(tables, current)
	}
	return tables
}

func cellKey(cell string) string {
	cell = strings.NewReplacer("<br>", " ", "<u>", "", "</u>", "", "¦", "|").Replace(cell)
	return strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) || character == '-' {
			return -1
		}
		return character
	}, cell)
}

func comparableRows(rows [][]string) [][]string {
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	used := make([]bool, width)
	var kept [][]string
	for _, row := range rows {
		keys := make([]string, width)
		empty := true
		for column, cell := range row {
			keys[column] = cellKey(cell)
			if keys[column] != "" {
				used[column] = true
				empty = false
			}
		}
		if !empty {
			kept = append(kept, keys)
		}
	}
	for index, row := range kept {
		var compact []string
		for column, key := range row {
			if used[column] {
				compact = append(compact, key)
			}
		}
		kept[index] = compact
	}
	return kept
}

func cellKeys(rows [][]string) map[string]bool {
	keys := map[string]bool{}
	for _, row := range comparableRows(rows) {
		for _, key := range row {
			if key != "" {
				keys[key] = true
			}
		}
	}
	return keys
}

func bestMatchingTable(tables [][][]string, expected [][]string) [][]string {
	wanted := cellKeys(expected)
	var best [][]string
	bestScore := 0.0
	for _, table := range tables {
		found := cellKeys(table)
		matched := 0
		for key := range found {
			if wanted[key] {
				matched++
			}
		}
		if score := 2 * float64(matched) / float64(len(wanted)+len(found)); score > bestScore {
			best, bestScore = table, score
		}
	}
	return best
}

func TestRealDocumentTables(t *testing.T) {
	paths, err := filepath.Glob("testdata/chpl/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		var document expectedDocument
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		t.Run(document.Document, func(t *testing.T) {
			tables := markdownTables(documentMarkdownOf(t, strings.TrimSuffix(path, ".json")+".pdf"))
			for _, table := range document.Tables {
				want := comparableRows(table.Rows)
				got := comparableRows(bestMatchingTable(tables, table.Rows))
				if len(got) != len(want) {
					t.Errorf("%s: got %d rows, want %d\ngot:  %q\nwant: %q", table.Name, len(got), len(want), got, want)
					continue
				}
				for index := range want {
					if strings.Join(got[index], "|") != strings.Join(want[index], "|") {
						t.Errorf("%s row %d:\ngot:  %q\nwant: %q", table.Name, index, got[index], want[index])
					}
				}
			}
		})
	}
}
