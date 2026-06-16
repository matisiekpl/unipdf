/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"github.com/matisiekpl/unipdf/v3/core"
	"github.com/matisiekpl/unipdf/v3/model"
)

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
// in the CJK (and beyond) Unicode ranges, which never legitimately appear in
// these Latin-script documents and signal a font whose code->unicode mapping is
// wrong.
func decodeBadness(texts []string, numMisses int) int {
	bad := numMisses
	for _, text := range texts {
		for _, r := range text {
			if r >= 0x2C00 {
				bad++
			}
		}
	}
	return bad
}
