/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"strings"

	"github.com/matisiekpl/unipdf/v3/core"
	"github.com/matisiekpl/unipdf/v3/internal/textencoding"
	"github.com/matisiekpl/unipdf/v3/model"
)

// symbolPUA maps Adobe Symbol font Private Use Area code points (U+F0xx =
// Symbol charcode 0xxx) to their real Unicode characters (≥, ≤, µ, α, β, …),
// which some PDFs encode in the PUA via their ToUnicode CMap. It is built once
// at startup so that lookups are read-only and safe for concurrent use (the
// Symbol encoder itself mutates internal state on each call).
var symbolPUA = buildSymbolPUA()

func buildSymbolPUA() map[rune]rune {
	encoder := textencoding.NewSymbolEncoder()
	out := make(map[rune]rune, 0x100)
	for code := 0; code <= 0xFF; code++ {
		// Skip codes the Symbol font itself maps into the Private Use Area
		// (e.g. bracket pieces); those have no real Unicode and are dropped.
		if mapped, ok := encoder.CharcodeToRune(textencoding.CharCode(code)); ok && mapped < 0xE000 {
			out[rune(0xF000+code)] = mapped
		}
	}
	// Dingbat (e.g. Wingdings) PUA code points the Symbol encoder does not cover.
	out[0xF09F] = '•'
	return out
}

// fixDecodedText repairs decoded strings that came from fonts with a broken
// code->unicode mapping: it remaps Symbol/dingbat Private Use Area code points
// to real characters and splits CJK code points that are really two ASCII bytes
// (the result of an Identity ToUnicode CMap reading byte pairs as Unicode).
func fixDecodedText(texts []string) {
	for i, text := range texts {
		needsFix := false
		for _, r := range text {
			if _, ok := repairRune(r); ok {
				needsFix = true
				break
			}
		}
		if !needsFix {
			continue
		}
		var b strings.Builder
		for _, r := range text {
			if repaired, ok := repairRune(r); ok {
				b.WriteString(repaired)
			} else {
				b.WriteRune(r)
			}
		}
		texts[i] = b.String()
	}
}

// repairRune returns the repaired text for a code point that came from a broken
// font mapping, and whether a repair applies. It handles Symbol/dingbat Private
// Use Area code points and CJK code points that are really one or two raw bytes
// (the result of an Identity ToUnicode CMap reading character codes as Unicode).
func repairRune(r rune) (string, bool) {
	if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
		// Control characters never legitimately appear in extracted display text;
		// they come from a broken mapping. Drop them.
		return "", true
	}
	if r >= 0xE000 && r <= 0xF8FF {
		// Private Use Area: map the known Symbol-font code points to real
		// characters; drop the rest (font-specific dingbat glyphs with no
		// standard meaning) so they don't show up as broken boxes.
		if mapped, ok := symbolPUA[r]; ok {
			return string(mapped), true
		}
		return "", true
	}
	if r >= 0x2E80 && r <= 0xFFFF {
		hi, hiOk := latinByteRune(byte(r >> 8))
		lo, loOk := latinByteRune(byte(r & 0xFF))
		hiNull := byte(r>>8) == 0
		loNull := byte(r&0xFF) == 0
		switch {
		case hiOk && loOk:
			return string(hi) + string(lo), true
		case hiOk && loNull:
			return string(hi), true
		case loOk && hiNull:
			return string(lo), true
		}
	}
	return "", false
}

// winAnsiByte maps a raw byte to the WinAnsi character it encodes (covering the
// 0x80–0x9F range that Latin-1 leaves as control characters, e.g. 0x96 = en
// dash). Built once at startup for concurrency-safe, read-only lookups.
var winAnsiByte = buildWinAnsiByte()

func buildWinAnsiByte() map[byte]rune {
	encoder := textencoding.NewWinAnsiEncoder()
	out := make(map[byte]rune, 0x100)
	for code := 0x20; code <= 0xFF; code++ {
		if r, ok := encoder.CharcodeToRune(textencoding.CharCode(code)); ok && r != 0xFFFD {
			out[byte(code)] = r
		}
	}
	return out
}

// latinByteRune returns the character a byte encodes (WinAnsi) when it is
// printable, used to recover text whose character codes an Identity ToUnicode
// CMap merged into a single code point.
func latinByteRune(b byte) (rune, bool) {
	r, ok := winAnsiByte[b]
	return r, ok
}

// siblingCandidates returns fonts that share `font`'s underlying typeface (same
// BaseFont once the subset tag is stripped) and could decode its character codes
// better. Some PDFs draw the same text with several font objects for one
// typeface, only some of which carry a usable ToUnicode/encoding.
func (to *textObject) siblingCandidates(font *model.PdfFont) []*model.PdfFont {
	if font == nil {
		return nil
	}
	key := fontSubsetKey(font.BaseFont())
	if key == "" {
		return nil
	}
	var out []*model.PdfFont
	for _, candidate := range to.e.fontSiblingPool()[key] {
		if candidate != font {
			out = append(out, candidate)
		}
	}
	return out
}

// fontSiblingPool returns (building once) a map from typeface key to the fonts
// that use it, gathered from the page resources and every nested form XObject.
func (e *Extractor) fontSiblingPool() map[string][]*model.PdfFont {
	if e.siblingPool != nil {
		return e.siblingPool
	}
	pool := map[string][]*model.PdfFont{}
	collectFontSiblings(e.resources, pool, map[*model.PdfPageResources]bool{})
	e.siblingPool = pool
	return pool
}

func collectFontSiblings(resources *model.PdfPageResources, pool map[string][]*model.PdfFont, visited map[*model.PdfPageResources]bool) {
	if resources == nil || visited[resources] {
		return
	}
	visited[resources] = true

	if fontDict, ok := core.GetDict(resources.Font); ok {
		for _, name := range fontDict.Keys() {
			font, err := model.NewPdfFontFromPdfObject(fontDict.Get(name))
			if err != nil || font == nil {
				continue
			}
			key := fontSubsetKey(font.BaseFont())
			if key != "" {
				pool[key] = append(pool[key], font)
			}
		}
	}

	if xobjectDict, ok := core.GetDict(resources.XObject); ok {
		for _, name := range xobjectDict.Keys() {
			form, err := resources.GetXObjectFormByName(name)
			if err != nil || form == nil || form.Resources == nil {
				continue
			}
			collectFontSiblings(form.Resources, pool, visited)
		}
	}
}

// fontSubsetKey strips the 6-letter subset tag (e.g. "RCNWUK+") from a BaseFont
// name so that differently subsetted instances of the same underlying font
// share a key. Returns the name unchanged when there is no subset tag.
func fontSubsetKey(baseFont string) string {
	if len(baseFont) > 7 && baseFont[6] == '+' {
		return baseFont[7:]
	}
	return baseFont
}

// decodeBadness scores how garbled a decode is: unmapped codes plus characters
// that never legitimately appear in these Latin-script documents — control
// characters, the replacement rune and code points in the CJK (and beyond)
// ranges — all of which signal a font whose code->unicode mapping is wrong.
func decodeBadness(texts []string, numMisses int) int {
	bad := numMisses
	for _, text := range texts {
		for _, r := range text {
			if r >= 0x2C00 || r == 0xFFFD || (r < 0x20 && r != '\t' && r != '\n') {
				bad++
			}
		}
	}
	return bad
}
