// Callbacks from the parser which update the screen.
// See also: https://invisible-island.net/xterm/ctlseqs/ctlseqs-contents.html (Escape sequences supported by xterm)
//           https://terminalguide.namepad.de/seq/

package terminal

import (
	"log"
	"strconv"

	"terminal_parser/ascii"
)

// TODO: Add a mode where we log unhandled escape sequences.

func (t *RichTextTerminal) printRune(r rune) {
	t.screen.print(r)
}

func (t *RichTextTerminal) handleCtrl(c byte) {
	switch c {
	case '\t':
		t.printRune('\t')
	case '\a':
	case '\b':
		t.screen.left(1) // We don't question things...
	case ascii.DEL:
		t.screen.backspace()
	case '\r':
		t.screen.cr()
	case '\n', '\f', '\v', ascii.NEL:
		t.screen.newline()
	}
}

func (t *RichTextTerminal) handleEsc(intermediates string, final byte) {
	if intermediates == "" {
		switch final {
		case 'c': // Full Reset (RIS)
			t.screen.newline()
			t.screen.resetAttributes()
		}
	}
}

func (t *RichTextTerminal) handleCSI(params []string, intermediates string, final byte) {
	if len(params) == 0 {
		panic("CSI handler received nil param slice")
	}

	var err error
	var nParams []int
	convertParamsWithDefault := func(defaultValue int) {
		nParams = make([]int, len(params))
		for i := 0; i < len(params); i++ {
			if params[i] == "" {
				nParams[i] = defaultValue
				continue
			}
			nParams[i], err = strconv.Atoi(params[i])
			if err != nil {
				log.Printf("params: %q, int: %q, final: %q", params, intermediates, final)
				panic("CSI handler received non-integer param")
				return
			}
		}
	}

	if intermediates == "" {
		switch final {
		case 'C': // Cursor Forward (CUF)
			convertParamsWithDefault(1)
			t.screen.right(nParams[0])
		case 'D': // Cursor Backward (CUB)
			convertParamsWithDefault(1)
			t.screen.left(nParams[0])
		case 'E': // Cursor Next Line (CNL)
			convertParamsWithDefault(1)
			t.screen.newlines(nParams[0])
		case 'J': // Erase in Display (ED)
			fallthrough
		case 'K': // Erase in Line (EL)
			convertParamsWithDefault(0)
			switch nParams[0] {
			case 0:
				t.screen.clearRight()
			case 1:
				t.screen.clearLeft()
			case 2:
				t.screen.clear()
			}
		case 'G': // Cursor Horizontal Absolute (CHA)
			convertParamsWithDefault(1)
			t.screen.setPos(0, nParams[0]-1)
		case 'H': // Cursor Position (CUP)
			convertParamsWithDefault(1)
			if len(params) == 1 {
				nParams = append(nParams, 1)
			}
			t.screen.setPos(nParams[0]-1, nParams[1]-1)
		case 'h': // Set Mode (SM)
			convertParamsWithDefault(0)
			if intermediates == "?" {
				for _, param := range nParams {
					switch param {
					case 47, 1049: // Alternate screen buffer, SMCUP
						t.upgrade()
					}
				}
			}
		case 'm': // Select Graphic Rendition (SGR)
			convertParamsWithDefault(0)
			for len(nParams) > 0 {
				handled := t.handleSGR(nParams)
				nParams = nParams[handled:]
			}
		}
	}
	if intermediates == "!" {
		switch final {
		case 'p': // Soft Terminal Reset
			t.screen.resetAttributes()
		}
	}
}

func (t *RichTextTerminal) handleSGR(nParams []int) (handled int) {
	handleColorSeq := func(setColor func(Color)) int {
		if len(nParams) >= 5 && nParams[1] == 2 {
			setColor(RGBColor{uint8(nParams[2]), uint8(nParams[3]), uint8(nParams[4])})
			return 5
		}
		if len(nParams) >= 3 && nParams[1] == 5 {
			setColor(ANSIColor(nParams[2]))
			return 3
		}
		return 1
	}

	switch nParams[0] {
	case 0:
		t.screen.resetAttributes() // TODO: apparently this isn't supposed to reset hyperlinks
	case 1:
		t.screen.setStyle(Bold)
	case 2:
		t.screen.setStyle(Dim)
	case 3:
		t.screen.setStyle(Italic)
	case 4:
		t.screen.setStyle(Underline)
	case 5, 6:
		t.screen.setStyle(Blink)
	case 7:
		t.screen.setStyle(Inverted)
	case 8:
		t.screen.setStyle(Hidden)
	case 9:
		t.screen.setStyle(Strikethrough)
	case 21:
		t.screen.resetStyle(Bold)
	case 22:
		t.screen.resetStyle(Dim)
	case 23:
		t.screen.resetStyle(Italic)
	case 24:
		t.screen.resetStyle(Underline)
	case 25:
		t.screen.resetStyle(Blink)
	case 27:
		t.screen.resetStyle(Inverted)
	case 28:
		t.screen.resetStyle(Hidden)
	case 29:
		t.screen.resetStyle(Strikethrough)
	case 38:
		return handleColorSeq(t.screen.setFg)
	case 39:
		t.screen.resetFg()
	case 48:
		return handleColorSeq(t.screen.setBg)
	case 49:
		t.screen.resetBg()
	case 73:
		t.screen.setStyle(Superscript)
		t.screen.resetStyle(Subscript)
	case 74:
		t.screen.setStyle(Subscript)
		t.screen.resetStyle(Superscript)
	case 75:
		t.screen.resetStyle(Superscript | Subscript)
	}
	switch {
	case nParams[0] >= 30 && nParams[0] <= 37:
		t.screen.setFg(ANSIColor(nParams[0] - 30))
	case nParams[0] >= 40 && nParams[0] <= 47:
		t.screen.setBg(ANSIColor(nParams[0] - 40))
	case nParams[0] >= 90 && nParams[0] <= 97:
		t.screen.setFg(ANSIColor(nParams[0] - 90 + 8))
	case nParams[0] >= 100 && nParams[0] <= 107:
		t.screen.setBg(ANSIColor(nParams[0] - 100 + 8))
	}
	return 1
}

func (t *RichTextTerminal) handleDSC(params []string, intermediates string, final byte) {}

func (t *RichTextTerminal) handleOSC(params []string) {
	if len(params) == 0 {
		return
	}
	switch params[0] {
	case "0": // Set Window Title
	case "7": // Set Working Directory
	case "8": // Hyperlink
		if len(params) < 3 {
			t.screen.resetURI()
			break
		}
		t.screen.setURI(params[2]) // includes "" to reset
	case "133": // Semantic Prompt (FinalTerm)
	case "633": // Shell Integration (VSCode)
	case "1337": // User Vars (iTerm2)
	}
}
