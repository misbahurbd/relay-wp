package pdf

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// localNotice is the stand-in's first line.
const localNotice = "Stand-in PDF: the real invoice layout renders through Cloudflare Browser Rendering."

// A4 in points, and the text block on it.
const (
	pageWidth    = 595
	pageHeight   = 842
	marginLeft   = 50
	firstLineY   = 792
	lineHeight   = 14
	linesPerPage = 52
	wrapAt       = 95
)

// localRenderer writes the HTML's text as a plain Helvetica PDF, without a
// browser, for development and tests. It keeps the words, not the layout.
type localRenderer struct{}

func (localRenderer) Render(_ context.Context, doc string) ([]byte, error) {
	lines := append([]string{localNotice, ""}, wrap(textLines(doc), wrapAt)...)
	var pages [][]string
	for len(lines) > 0 {
		n := min(linesPerPage, len(lines))
		pages = append(pages, lines[:n])
		lines = lines[n:]
	}
	return writePDF(pages), nil
}

var (
	invisible  = regexp.MustCompile(`(?is)<(head|style|script)\b.*?</(head|style|script)\s*>`)
	lineBreaks = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|tr|li|h[1-6]|table|section|header|footer)\s*>`)
	cellBreaks = regexp.MustCompile(`(?i)</t[dh]\s*>`)
	tags       = regexp.MustCompile(`<[^>]*>`)
)

// textLines is the document's visible text, one non-empty line per block.
func textLines(doc string) []string {
	doc = invisible.ReplaceAllString(doc, "")
	doc = lineBreaks.ReplaceAllString(doc, "\n")
	doc = cellBreaks.ReplaceAllString(doc, " ")
	doc = html.UnescapeString(tags.ReplaceAllString(doc, ""))
	var out []string
	for line := range strings.SplitSeq(doc, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// wrap breaks lines longer than width runes at spaces.
func wrap(lines []string, width int) []string {
	var out []string
	for _, line := range lines {
		cur := ""
		for _, word := range strings.Fields(line) {
			if cur != "" && len([]rune(cur))+1+len([]rune(word)) > width {
				out = append(out, cur)
				cur = ""
			}
			if cur != "" {
				cur += " "
			}
			cur += word
		}
		out = append(out, cur)
	}
	return out
}

// writePDF lays out one text page per entry. Objects: 1 catalog, 2 pages,
// 3 font, then a page and its content stream for each page.
func writePDF(pages [][]string) []byte {
	var objs []string
	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	objs = append(objs,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	)
	for i, lines := range pages {
		var content strings.Builder
		fmt.Fprintf(&content, "BT /F1 10 Tf %d TL %d %d Td\n", lineHeight, marginLeft, firstLineY)
		for _, l := range lines {
			fmt.Fprintf(&content, "(%s) Tj T*\n", pdfString(l))
		}
		content.WriteString("ET")
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>",
				pageWidth, pageHeight, 5+2*i),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		)
	}

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// winAnsi maps the few non-Latin-1 characters invoices use to WinAnsiEncoding.
var winAnsi = map[rune]byte{'€': 0x80, '–': 0x96, '—': 0x97, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95}

// pdfString escapes s for a PDF literal string in WinAnsiEncoding; anything
// it can't encode becomes "?".
func pdfString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r >= 0x20 && r < 0x7f:
			b.WriteRune(r)
		case r >= 0xa0 && r <= 0xff:
			fmt.Fprintf(&b, "\\%03o", r)
		default:
			if c, ok := winAnsi[r]; ok {
				fmt.Fprintf(&b, "\\%03o", c)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}
