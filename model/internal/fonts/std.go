/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package fonts

import (
	"github.com/oliverpool/unipdf/v3/core"
	"github.com/oliverpool/unipdf/v3/internal/textencoding"
)

// StdFontName is a name of a standard font.
type StdFontName string

// FontWeight specified font weight.
type FontWeight int

// Font weights
const (
	FontWeightMedium FontWeight = iota // Medium
	FontWeightBold                     // Bold
	FontWeightRoman                    // Roman
)

// Descriptor describes geometric properties of a font.
type Descriptor struct {
	Name        StdFontName
	Family      string
	Weight      FontWeight
	Flags       uint
	BBox        [4]float64
	ItalicAngle float64
	Ascent      float64
	Descent     float64
	CapHeight   float64
	XHeight     float64
	StemV       float64
	StemH       float64
}

var stdFonts = make(map[StdFontName]func() StdFont)

// IsStdFont check if a name is registered for a standard font.
func IsStdFont(name StdFontName) bool {
	_, ok := stdFonts[name]
	return ok
}

// NewStdFontByName creates a new StdFont by registered name. See RegisterStdFont.
func NewStdFontByName(name StdFontName) (StdFont, bool) {
	fnc, ok := stdFonts[name]
	if !ok {
		return StdFont{}, false
	}
	return fnc(), true
}

// RegisterStdFont registers a given StdFont constructor by font name. Font can then be created with NewStdFontByName.
func RegisterStdFont(name StdFontName, fnc func() StdFont, aliases ...StdFontName) {
	if _, ok := stdFonts[name]; ok {
		panic("font already registered: " + string(name))
	}
	stdFonts[name] = fnc
	for _, alias := range aliases {
		RegisterStdFont(alias, fnc)
	}
}

var _ Font = StdFont{}

// StdFont represents one of the built-in fonts and it is assumed that every reader has access to it.
type StdFont struct {
	desc    Descriptor
	metrics map[rune]CharMetrics
	encoder textencoding.TextEncoder
}

// NewStdFont returns a new instance of the font with a default encoder set (StandardEncoding).
func NewStdFont(desc Descriptor, metrics map[rune]CharMetrics) StdFont {
	return NewStdFontWithEncoding(desc, metrics, textencoding.NewStandardEncoder())
}

// NewStdFontWithEncoding returns a new instance of the font with a specified encoder.
func NewStdFontWithEncoding(desc Descriptor, metrics map[rune]CharMetrics, encoder textencoding.TextEncoder) StdFont {
	var nbsp rune = 0xA0
	if _, ok := metrics[nbsp]; !ok {
		// Use same metrics for 0xA0 (no-break space) and 0x20 (space).
		metrics[nbsp] = metrics[0x20]
	}

	return StdFont{
		desc:    desc,
		metrics: metrics,
		encoder: encoder,
	}
}

// Name returns a PDF name of the font.
func (font StdFont) Name() string {
	return string(font.desc.Name)
}

// Encoder returns the font's text encoder.
func (font StdFont) Encoder() textencoding.TextEncoder {
	return font.encoder
}

// GetRuneMetrics returns character metrics for a given rune.
func (font StdFont) GetRuneMetrics(r rune) (CharMetrics, bool) {
	metrics, has := font.metrics[r]
	return metrics, has
}

// GetMetricsTable is a method specific to standard fonts. It returns the metrics table of all glyphs.
// Caller should not modify the table.
func (font StdFont) GetMetricsTable() map[rune]CharMetrics {
	return font.metrics
}

// Descriptor returns a font descriptor.
func (font StdFont) Descriptor() Descriptor {
	return font.desc
}

// ToPdfObject returns a primitive PDF object representation of the font.
func (font StdFont) ToPdfObject() core.PdfObject {
	fontDict := core.MakeDict()
	fontDict.Set("Type", core.MakeName("Font"))
	fontDict.Set("Subtype", core.MakeName("Type1"))
	fontDict.Set("BaseFont", core.MakeName(font.Name()))
	fontDict.Set("Encoding", font.encoder.ToPdfObject())

	return core.MakeIndirectObject(fontDict)
}

// type1CommonRunes is list of runes common for some Type1 fonts. Used to unpack character metrics.
var type1CommonRunes = []rune{
	'A', 'Æ', 'Á', 'Ă', 'Â', 'Ä', 'À', 'Ā', 'Ą', 'Å',
	'Ã', 'B', 'C', 'Ć', 'Č', 'Ç', 'D', 'Ď', 'Đ', '∆',
	'E', 'É', 'Ě', 'Ê', 'Ë', 'Ė', 'È', 'Ē', 'Ę', 'Ð',
	'€', 'F', 'G', 'Ğ', 'Ģ', 'H', 'I', 'Í', 'Î', 'Ï',
	'İ', 'Ì', 'Ī', 'Į', 'J', 'K', 'Ķ', 'L', 'Ĺ', 'Ľ',
	'Ļ', 'Ł', 'M', 'N', 'Ń', 'Ň', 'Ņ', 'Ñ', 'O', 'Œ',
	'Ó', 'Ô', 'Ö', 'Ò', 'Ő', 'Ō', 'Ø', 'Õ', 'P', 'Q',
	'R', 'Ŕ', 'Ř', 'Ŗ', 'S', 'Ś', 'Š', 'Ş', 'Ș', 'T',
	'Ť', 'Ţ', 'Þ', 'U', 'Ú', 'Û', 'Ü', 'Ù', 'Ű', 'Ū',
	'Ų', 'Ů', 'V', 'W', 'X', 'Y', 'Ý', 'Ÿ', 'Z', 'Ź',
	'Ž', 'Ż', 'a', 'á', 'ă', 'â', '´', 'ä', 'æ', 'à',
	'ā', '&', 'ą', 'å', '^', '~', '*', '@', 'ã', 'b',
	'\\', '|', '{', '}', '[', ']', '˘', '¦', '•', 'c',
	'ć', 'ˇ', 'č', 'ç', '¸', '¢', 'ˆ', ':', ',', '\uf6c3',
	'©', '¤', 'd', '†', '‡', 'ď', 'đ', '°', '¨', '÷',
	'$', '˙', 'ı', 'e', 'é', 'ě', 'ê', 'ë', 'ė', 'è',
	'8', '…', 'ē', '—', '–', 'ę', '=', 'ð', '!', '¡',
	'f', 'ﬁ', '5', 'ﬂ', 'ƒ', '4', '⁄', 'g', 'ğ', 'ģ',
	'ß', '`', '>', '≥', '«', '»', '‹', '›', 'h', '˝',
	'-', 'i', 'í', 'î', 'ï', 'ì', 'ī', 'į', 'j', 'k',
	'ķ', 'l', 'ĺ', 'ľ', 'ļ', '<', '≤', '¬', '◊', 'ł',
	'm', '¯', '−', 'µ', '×', 'n', 'ń', 'ň', 'ņ', '9',
	'≠', 'ñ', '#', 'o', 'ó', 'ô', 'ö', 'œ', '˛', 'ò',
	'ő', 'ō', '1', '½', '¼', '¹', 'ª', 'º', 'ø', 'õ',
	'p', '¶', '(', ')', '∂', '%', '.', '·', '‰', '+',
	'±', 'q', '?', '¿', '"', '„', '“', '”', '‘', '’',
	'‚', '\'', 'r', 'ŕ', '√', 'ř', 'ŗ', '®', '˚', 's',
	'ś', 'š', 'ş', 'ș', '§', ';', '7', '6', '/', ' ',
	'£', '∑', 't', 'ť', 'ţ', 'þ', '3', '¾', '³', '˜',
	'™', '2', '²', 'u', 'ú', 'û', 'ü', 'ù', 'ű', 'ū',
	'_', 'ų', 'ů', 'v', 'w', 'x', 'y', 'ý', 'ÿ', '¥',
	'z', 'ź', 'ž', 'ż', '0',
}
