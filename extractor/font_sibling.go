/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"strings"

	"github.com/matisiekpl/unipdf/v3/common"
	"github.com/matisiekpl/unipdf/v3/core"
	"github.com/matisiekpl/unipdf/v3/model"
)

// siblingDecoder returns a font that shares `font`'s embedded subset (same
// BaseFont) but is more likely to decode its character codes — typically a
// Type0 (CID) sibling carrying a complete ToUnicode CMap. It returns nil when no
// such sibling exists. Some PDFs draw the same text with two font objects for
// one embedded subset, only one of which has a usable ToUnicode map.
func (to *textObject) siblingDecoder(font *model.PdfFont) *model.PdfFont {
	if font == nil {
		return nil
	}
	key := fontSubsetKey(font.BaseFont())
	if key == "" {
		return nil
	}
	sibling := to.e.fontSiblingPool()[key]
	if sibling == nil || sibling == font {
		return nil
	}
	return sibling
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

// fontSiblingPool returns (building once) a map from BaseFont subset name to a
// Type0 font that uses that subset, gathered from the page resources and every
// nested form XObject. Type0 fonts read multi-byte codes and tend to carry the
// complete ToUnicode CMap, so they can decode codes that a sibling simple font
// fails on.
func (e *Extractor) fontSiblingPool() map[string]*model.PdfFont {
	if e.siblingPool != nil {
		return e.siblingPool
	}
	pool := map[string]*model.PdfFont{}
	collectFontSiblings(e.resources, pool, map[*model.PdfPageResources]bool{})
	e.siblingPool = pool
	return pool
}

func collectFontSiblings(resources *model.PdfPageResources, pool map[string]*model.PdfFont, visited map[*model.PdfPageResources]bool) {
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
			if key == "" || !strings.HasPrefix(font.Subtype(), "Type0") {
				continue
			}
			if _, exists := pool[key]; !exists {
				pool[key] = font
			}
		}
	}

	xobjectDict, ok := core.GetDict(resources.XObject)
	if !ok {
		return
	}
	for _, name := range xobjectDict.Keys() {
		form, err := resources.GetXObjectFormByName(name)
		if err != nil || form == nil || form.Resources == nil {
			continue
		}
		collectFontSiblings(form.Resources, pool, visited)
	}
	common.Log.Trace("collectFontSiblings: pool size=%d", len(pool))
}
