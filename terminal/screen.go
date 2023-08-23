package terminal

import (
	"sync"
)

type styleFlags uint32

const (
	Bold styleFlags = 1 << iota
	Dim
	Italic
	Underline
	Blink
	Inverted
	Hidden
	Strikethrough
	DoubleUnderline
	Superscript
	Subscript
)

type styleAttributes struct {
	styleFlags
	// NOTE: A nil color represents the default value
	fg        Color
	bg        Color
	underline Color

	uri string
}

func (a *styleAttributes) Equals(a2 *styleAttributes) bool {
	return a == a2 || *a == *a2
}

func (a *styleAttributes) Empty() bool {
	return *a == styleAttributes{}
}

func (a *styleAttributes) hasStyle(flags styleFlags) bool {
	return a.styleFlags&flags != 0
}

type node struct {
	rune
	*styleAttributes
}

type screen struct {
	scrollback [][]node // TODO: could also just be slice of rendered HTML segments, one for each line :)

	activeLine []node
	pos        int

	activeAttributes *styleAttributes

	sync.Mutex // TODO
}

func newScreen() screen {
	return screen{
		activeAttributes: &styleAttributes{},
	}
}

func (s *screen) print(r rune) {
	if s.pos < len(s.activeLine) {
		s.activeLine[s.pos] = node{r, s.activeAttributes}
	} else {
		s.activeLine = append(s.activeLine, node{r, s.activeAttributes})
	}

	s.pos++
}

func (s *screen) backspace() {
	if len(s.activeLine) == 0 {
		return
	}

	s.activeLine = append(s.activeLine[:s.pos-1], s.activeLine[s.pos:]...)
	s.pos--
}

func (s *screen) newline() {
	s.scrollback = append(s.scrollback, s.activeLine)
	s.activeLine = nil
	s.pos = 0
}

func (s *screen) left(n int) {
	s.setPos(0, s.pos-n)
}

func (s *screen) right(n int) {
	s.setPos(0, s.pos+n)
}

func (s *screen) cr() {
	s.pos = 0
}

func (s *screen) newlines(n int) {
	for i := 0; i < n; i++ {
		s.newline()
	}
}

func (s *screen) setPos(x, y int) {
	if y < 0 {
		s.pos = 0
		return
	}
	for i := len(s.activeLine); i < y; i++ {
		s.print(' ')
	}
	s.pos = y
}

func (s *screen) clear() {
	s.activeLine = []node{}
	s.pos = 0
}

func (s *screen) clearLeft() {
	for i := 0; i < s.pos; i++ {
		s.activeLine[i] = node{' ', s.activeAttributes}
	}
}

func (s *screen) clearRight() {
	s.activeLine = s.activeLine[:s.pos]
}

func (s *screen) copyAttributes() {
	cpy := *s.activeAttributes
	s.activeAttributes = &cpy
}

func (s *screen) resetAttributes() {
	s.activeAttributes = &styleAttributes{}
}

func (s *screen) setStyle(flags styleFlags) {
	s.copyAttributes()
	s.activeAttributes.styleFlags |= flags
}

func (s *screen) resetStyle(flags styleFlags) {
	s.copyAttributes()
	s.activeAttributes.styleFlags &= ^flags
}

func (s *screen) setFg(color Color) {
	s.copyAttributes()
	s.activeAttributes.fg = color
}

func (s *screen) setBg(color Color) {
	s.copyAttributes()
	s.activeAttributes.bg = color
}

func (s *screen) resetFg() {
	s.setFg(nil)
}

func (s *screen) resetBg() {
	s.setBg(nil)
}

func (s *screen) setURI(uri string) {
	s.copyAttributes()
	s.activeAttributes.uri = uri
}

func (s *screen) resetURI() {
	s.copyAttributes()
	s.activeAttributes.uri = ""
}
