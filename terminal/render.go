package terminal

import (
	"fmt"
	"html"
	"strings"
)

func renderLine(line []node) string {
	var raw strings.Builder

	openTags := func(attr *styleAttributes) {
		if attr.uri != "" {
			fmt.Fprintf(&raw, "<a href=\"%s\">", html.EscapeString(attr.uri))
		}
		if !attr.Empty() {
			raw.WriteString("<span style=\"")
			if attr.hasStyle(Bold) {
				raw.WriteString("font-weight:bold;")
			}
			if attr.hasStyle(Italic) {
				raw.WriteString("font-style:italic;")
			}
			if attr.hasStyle(Underline) {
				raw.WriteString("text-decoration:underline;")
			}
			if attr.hasStyle(Hidden) {
				raw.WriteString("visibility:hidden;")
			}
			if attr.fg != nil {
				fmt.Fprintf(&raw, "color:%s;", attr.fg.HTMLColorCode())
			}
			if attr.bg != nil {
				fmt.Fprintf(&raw, "background-color:%s;", attr.bg.HTMLColorCode())
			}
			raw.WriteString("\">")
		}
	}
	closeTags := func(attr *styleAttributes) {
		if !attr.Empty() {
			raw.WriteString("</span>")
		}
		if attr.uri != "" {
			fmt.Fprintf(&raw, "</a>")
		}
	}

	prevAttr := &styleAttributes{}
	for _, n := range line {
		if !n.styleAttributes.Equals(prevAttr) {
			closeTags(prevAttr)
			openTags(n.styleAttributes)
			prevAttr = n.styleAttributes
		}

		//raw.WriteRune(n.rune)
		raw.WriteString(html.EscapeString(string(n.rune)))
	}
	closeTags(prevAttr)

	return raw.String()
}

func (s *screen) Lines() []string {
	// TODO: obvious race condition
	ret := make([]string, 0, len(s.scrollback)+1)
	for _, line := range append(s.scrollback, s.activeLine) {
		ret = append(ret, renderLine(line))
	}
	return ret
}
