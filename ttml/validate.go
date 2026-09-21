// Package ttml validates that an uploaded file is a well-formed, sane TTML
// lyrics document before it's ever persisted (docs/ARCHITECTURE.md §10.3).
package ttml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

const Namespace = "http://www.w3.org/ns/ttml"

// clockTime matches TTML clock-time ("1:02:03.456", "02:03.456") and simple
// offset-time ("12.34s") forms - enough to sanity-check begin/end ordering
// without implementing the full TTML time-expression grammar.
var clockTime = regexp.MustCompile(`^(?:(\d+):)?(?:(\d+):)?(\d+(?:\.\d+)?)(s)?$`)

func parseTime(v string) (float64, bool) {
	m := clockTime.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return 0, false
	}
	var hours, minutes float64
	if m[1] != "" && m[2] != "" {
		hours, _ = strconv.ParseFloat(m[1], 64)
		minutes, _ = strconv.ParseFloat(m[2], 64)
	} else if m[1] != "" {
		minutes, _ = strconv.ParseFloat(m[1], 64)
	}
	seconds, _ := strconv.ParseFloat(m[3], 64)
	return hours*3600 + minutes*60 + seconds, true
}

type paragraph struct {
	XMLName xml.Name `xml:"p"`
	Begin   string   `xml:"begin,attr"`
	End     string   `xml:"end,attr"`
}

type body struct {
	Paragraphs []paragraph `xml:"div>p"`
	// TTML allows <p> directly under <body> too, without a <div>.
	DirectParagraphs []paragraph `xml:"p"`
}

type tt struct {
	XMLName xml.Name `xml:"tt"`
	Body    body     `xml:"body"`
}

// Validate returns an error describing exactly what's wrong, or nil if data
// is an acceptable TTML lyrics document.
func Validate(data []byte, maxParagraphs int) error {
	if bytes.Contains(bytes.ToUpper(data), []byte("<!DOCTYPE")) {
		return fmt.Errorf("DOCTYPE declarations are not allowed")
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true

	var doc tt
	if err := decoder.Decode(&doc); err != nil {
		return fmt.Errorf("not well-formed XML: %w", err)
	}

	if doc.XMLName.Local != "tt" {
		return fmt.Errorf("root element must be <tt>, got <%s>", doc.XMLName.Local)
	}
	if doc.XMLName.Space != "" && doc.XMLName.Space != Namespace {
		return fmt.Errorf("root element must be in namespace %s, got %s", Namespace, doc.XMLName.Space)
	}

	paragraphs := append(append([]paragraph{}, doc.Body.Paragraphs...), doc.Body.DirectParagraphs...)
	if len(paragraphs) == 0 {
		return fmt.Errorf("no timed <p> elements found in <body>")
	}
	if maxParagraphs > 0 && len(paragraphs) > maxParagraphs {
		return fmt.Errorf("too many <p> elements (%d, max %d)", len(paragraphs), maxParagraphs)
	}

	for i, p := range paragraphs {
		if p.Begin == "" || p.End == "" {
			continue // untimed lines are allowed (e.g. static header text)
		}
		begin, ok := parseTime(p.Begin)
		if !ok {
			return fmt.Errorf("paragraph %d: unparseable begin time %q", i, p.Begin)
		}
		end, ok := parseTime(p.End)
		if !ok {
			return fmt.Errorf("paragraph %d: unparseable end time %q", i, p.End)
		}
		if end <= begin {
			return fmt.Errorf("paragraph %d: end (%s) must be after begin (%s)", i, p.End, p.Begin)
		}
	}
	return nil
}

// extractParagraph captures a <p>'s raw inner markup so its flattened text
// content can be recovered regardless of whether it's word-synced (nested
// <span> per word) or line-synced (plain text directly in the <p>).
type extractParagraph struct {
	XMLName xml.Name `xml:"p"`
	Begin   string   `xml:"begin,attr"`
	End     string   `xml:"end,attr"`
	Inner   string   `xml:",innerxml"`
}

type extractBody struct {
	Paragraphs []extractParagraph `xml:"div>p"`
	// TTML allows <p> directly under <body> too, without a <div>.
	DirectParagraphs []extractParagraph `xml:"p"`
}

type extractTT struct {
	XMLName xml.Name    `xml:"tt"`
	Body    extractBody `xml:"body"`
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// plainTextFromInner strips any child markup (e.g. per-word/per-syllable
// <span> tags used by word-synced lyrics) from a <p>'s inner XML, leaving
// flattened, decoded text with normalized whitespace.
//
// Word-synced TTML as produced by common tooling pretty-prints one <span>
// per line, indented with tabs/newlines - including between syllable spans
// that make up a single word (no source-level separator between them).
// A literal, single space character on its own line is used as the actual
// inter-word separator. So: drop indentation whitespace (tabs/newlines)
// entirely rather than converting it to a space, and only collapse runs of
// real space characters.
func plainTextFromInner(inner string) string {
	stripped := tagPattern.ReplaceAllString(inner, "")
	unescaped := html.UnescapeString(stripped)
	noIndent := strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(unescaped)
	return strings.Join(strings.Fields(noIndent), " ")
}

// formatLRCTime renders a paragraph begin time as LRC-style "mm:ss.xx".
func formatLRCTime(totalSeconds float64) string {
	minutes := int(totalSeconds) / 60
	seconds := totalSeconds - float64(minutes*60)
	return fmt.Sprintf("%02d:%05.2f", minutes, seconds)
}

// ExtractText parses a TTML lyrics document and returns two representations
// of its text: plain lyrics (one line per <p>, newline separated) and
// LRC-style synced lyrics ("[mm:ss.xx] text" per line, using each
// paragraph's begin time). Word-synced TTML (nested <span> per word) and
// line-synced TTML are handled uniformly since only each <p>'s begin time
// and flattened text content are used - per-word timing is not preserved in
// either output.
func ExtractText(data []byte) (plain string, synced string, err error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true

	var doc extractTT
	if err := decoder.Decode(&doc); err != nil {
		return "", "", fmt.Errorf("not well-formed XML: %w", err)
	}

	paragraphs := append(append([]extractParagraph{}, doc.Body.Paragraphs...), doc.Body.DirectParagraphs...)

	var plainLines, syncedLines []string
	for _, p := range paragraphs {
		text := plainTextFromInner(p.Inner)
		if text == "" {
			continue
		}
		plainLines = append(plainLines, text)

		if begin, ok := parseTime(p.Begin); ok {
			syncedLines = append(syncedLines, fmt.Sprintf("[%s]%s", formatLRCTime(begin), text))
		} else {
			syncedLines = append(syncedLines, text)
		}
	}

	return strings.Join(plainLines, "\n"), strings.Join(syncedLines, "\n"), nil
}
