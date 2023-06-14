# UniPDF - PDF for Go - AGPL Fork

[UniDoc](http://unidoc.io)'s UniPDF (formerly unidoc) is a PDF library for Go (golang) with capabilities for
creating and reading, processing PDF files. The library is written and supported by
[FoxyUtils.com](https://foxyutils.com), where the library is used to power many of its services.

[![Build Status](https://app.wercker.com/status/22b50db125a6d376080f3f0c80d085fa/s/master "wercker status")](https://app.wercker.com/project/bykey/22b50db125a6d376080f3f0c80d085fa)
[![GitHub (pre-)release](https://img.shields.io/github/release/unidoc/unipdf/all.svg)](https://github.com/oliverpool/unipdf/releases)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/oliverpool/unipdf)](https://goreportcard.com/report/github.com/oliverpool/unipdf)
[![GoDoc](https://godoc.org/github.com/oliverpool/unipdf?status.svg)](https://godoc.org/github.com/oliverpool/unipdf)

## Features

- [Create PDF reports](https://github.com/unidoc/unipdf-examples/blob/v3/report/pdf_report.go). Example output: [unidoc-report.pdf](https://github.com/unidoc/unipdf-examples/blob/v3/report/unidoc-report.pdf).
- [Table PDF reports](https://github.com/unidoc/unipdf-examples/blob/v3/report/pdf_tables.go). Example output: [unipdf-tables.pdf](https://github.com/unidoc/unipdf-examples/blob/v3/report/unipdf-tables.pdf).
- [Invoice creation](https://unidoc.io/news/simple-invoices)
- Paragraph in creator handling multiple styles within the same paragraph
- [Merge PDF pages](https://github.com/unidoc/unipdf-examples/blob/v3/pages/pdf_merge.go)
- [Split PDF pages](https://github.com/unidoc/unipdf-examples/blob/v3/pages/pdf_split.go) and change page order
- [Rotate pages](https://github.com/unidoc/unipdf-examples/blob/v3/pages/pdf_rotate.go)
- [Extract text from PDF files](https://github.com/unidoc/unipdf-examples/blob/v3/text/pdf_extract_text.go)
- [Text extraction support with size, position and formatting info](https://github.com/unidoc/unipdf-examples/blob/v3/text/pdf_text_locations.go)
- [PDF to CSV](https://github.com/unidoc/unipdf-examples/blob/v3/text/pdf_to_csv.go) illustrates extracting tabular data from PDF.
- [Extract images](https://github.com/unidoc/unipdf-examples/blob/v3/image/pdf_extract_images.go) with coordinates
- [Images to PDF](https://github.com/unidoc/unipdf-examples/blob/v3/image/pdf_images_to_pdf.go)
- [Add images to pages](https://github.com/unidoc/unipdf-examples/blob/v3/image/pdf_add_image_to_page.go)
- [Compress and optimize PDF](https://github.com/unidoc/unipdf-examples/blob/v3/compress/pdf_optimize.go)
- [Watermark PDF files](https://github.com/unidoc/unipdf-examples/blob/v3/image/pdf_watermark_image.go)
- Advanced page manipulation (blocks/templates)
- Load PDF templates and modify
- [Form creation](https://github.com/unidoc/unipdf-examples/blob/v3/forms/pdf_form_add.go)
- [Fill and flatten forms](https://github.com/unidoc/unipdf-examples/blob/v3/forms/pdf_form_flatten.go)
- [Fill out forms](https://github.com/unidoc/unipdf-examples/blob/v3/forms/pdf_form_fill_json.go) and [FDF merging](https://github.com/unidoc/unipdf-examples/blob/v3/forms/pdf_form_fill_fdf_merge.go)
- [Unlock PDF files / remove password](https://github.com/unidoc/unipdf-examples/blob/v3/security/pdf_unlock.go)
- [Protect PDF files with a password](https://github.com/unidoc/unipdf-examples/blob/v3/security/pdf_protect.go)
- [Digital signing validation and signing](https://github.com/unidoc/unipdf-examples/tree/v3/signatures)
- CCITTFaxDecode decoding and encoding support
- JBIG2 decoding support

Multiple examples are provided in our example repository https://github.com/unidoc/unidoc-examples.


## Installation
With modules:
~~~
go get github.com/oliverpool/unipdf/v3
~~~

## Licensing Information

This library (unipdf) is available under an AGPL license that can be used in AGPL-compatible open source software.

Please visit [unidoc.io](https://unidoc.io/unipdf/) for other usages.
