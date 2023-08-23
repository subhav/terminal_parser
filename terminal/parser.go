// Tokenize ANSI/VT100-style escape sequences for a terminal emulator.
//
// The parser runs independently and pushes tokens to the terminal through the callbacks in the dispatchHandler
// interface.
//
// See https://vt100.net/emu/dec_ansi_parser for a specification, with state and action definitions, including for
// terminals that supported 8-bit control codes (C1) for non-Unicode encodings.  This file loosely tracks this spec,
// but we currently don't support 8-bit C1 controls or Unicode-encoded C1 controls (U+0080 to U+009F).
//
// See also: https://en.wikipedia.org/wiki/ANSI_escape_code,  https://en.wikipedia.org/wiki/C0_and_C1_control_codes,
//           https://en.wikipedia.org/wiki/Latin-1_Supplement,

package terminal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"terminal_parser/ascii"
)

// dispatchHandler includes callbacks for a terminal emulator to implement.
type dispatchHandler interface {
	printRune(rune)
	handleCtrl(byte)
	handleEsc(intermediates string, final byte)
	handleOSC(params []string)
	handleCSI(params []string, intermediates string, final byte)
	handleDSC(params []string, intermediates string, final byte)
}

type state func(p *parser) (state, error)

func (s state) String() string {
	return runtime.FuncForPC(reflect.ValueOf(s).Pointer()).Name()
}

func (s state) Equals(s2 state) bool {
	// Go doesn't allow comparing functions, unfortunately
	return reflect.ValueOf(s).Pointer() == reflect.ValueOf(s2).Pointer()
}

var parserPaused = errors.New("parser state machine paused")

func parserUndefinedTransition(c byte, s state) error {
	return fmt.Errorf("parser is missing a transition on byte %q from state %s", c, s)
}

type parser struct {
	dispatchHandler

	rd io.Reader
	// TODO: Using bufio.Reader *may* break upgradability.
	//       As long as there's data to read and space in the buffer, bufio.Reader will read into it.
	//       This means that when we decide we need to perform an upgrade to an external terminal emulator,
	//       we may have already read past the escape sequence that prompted us to do that.
	//
	//  It might be smart for us to do this ourselves instead. A couple options:
	//  (1) parser.Continue() takes a byte as an input and advances the state machine one step. Other similar libraries
	//      work using []byte, advancing until the byte slice is parsed.
	//  (2) We create our own buffer and only read one byte at a time into it. This way, we can also always keep an
	//      entire escape sequence in the buffer, allowing us to reduce allocations and copies when building params.
	buf *bufio.Reader

	state state

	partialParam         strings.Builder
	partialParams        []string
	partialIntermediates strings.Builder
}

func newParser(src io.Reader, handler dispatchHandler) *parser {
	return &parser{
		rd:              src,
		buf:             bufio.NewReader(src),
		dispatchHandler: handler,

		state: parseOutput,
	}
}

func ctrlCode(c byte) bool {
	return c <= 0x1f && !(c == ascii.CAN || c == ascii.SUB || c == ascii.ESC)
}

func terminatingCtrlCode(c byte) bool {
	return c == ascii.CAN || c == ascii.SUB
}

func graphicalCode(c byte) bool {
	return c >= 0x20 // includes DEL (0x7f)
}

// Continue runs the state machine until the next dispatch event completes
// (For the most part... Extra handleCtrls might get called first.)
func (p *parser) Continue() error {
	var lastState state = nil
	var err error
	// Kind of funky control flow, each state returns to the next state.
	for p.state != nil && err == nil {
		// Check common transitions on entry to any state.
		var c byte
		c, err = p.buf.ReadByte()
		if err != nil {
			break
		}

		switch {
		case ctrlCode(c):
			p.handleCtrl(c)
		case terminatingCtrlCode(c):
			p.handleCtrl(c)
			return err
		case c == 0x1b:
			p.state = parseEscape
		case c >= 0x80 && c < 0xc0: // Not a valid utf-8 first byte
			// ignore as a hack to avoid many undefined transitions
		default:
			// Assume that if we try to enter the same state twice without pausing the state machine,
			// we're caught in an infinite loop.
			// TODO: state.Equals is expensive and possibly inaccurate, and this is in the critical path.
			//       Move the common transitions down into to each state function after all.
			if p.state.Equals(lastState) {
				return parserUndefinedTransition(c, p.state)
			}
			_ = p.buf.UnreadByte()
		}

		//log.Printf("parser: entering state %q with byte %q", p.state, c)
		lastState = p.state
		p.state, err = p.state(p)
		//log.Printf("parser: next state %q", p.state)
	}

	if err == parserPaused {
		return nil
	}
	return err
}

func (p *parser) Run(ctx context.Context) {
	var err error
	for err == nil {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err = p.Continue()
	}

	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
		log.Printf("parser.Run exited with: %v", err)
	}
}

// parseOutput implements the "ground" state
func parseOutput(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case c == ascii.DEL:
			p.handleCtrl(c)
			return parseOutput, parserPaused
		case graphicalCode(c):
			_ = p.buf.UnreadByte()
			// TODO: Read until the next non-text character and printString instead.
			//       Would need to make reads non-blocking to exit early.
			r, _, err := p.buf.ReadRune()
			if err != nil {
				return nil, fmt.Errorf("ReadRune failed with: %w", err)
			}
			p.printRune(r)
			// TODO: Can we avoid returning right away if there's more data to read?
			// return parseOutput, parserPaused
		default:
			_ = p.buf.UnreadByte()
			return parseOutput, parserPaused
		}
	}
}

// parseEscape begins parsing an escape sequence
func parseEscape(p *parser) (state, error) {
	c, err := p.buf.ReadByte()
	if err != nil {
		return nil, err
	}
	var low = c &^ 0x80

	switch {
	case c == ascii.DEL:
		// ignore
	case c >= 0x20 && c <= 0x2f:
		p.clear()
		p.collectIntermediate(c)
		return parseEscapeIntermediate, nil
	case low == 'P':
		return parseDCSEntry, nil
	case low == '[':
		return parseCSIEntry, nil
	case low == ']':
		return parseOSCString, nil
	// SOS, PM, APC
	case low == 'X' || low == '^' || low == '_':
		return parseIgnoreAll, nil
	// String Terminator (ST) is a no-op, no need to dispatch
	case low == '\\':
		return parseOutput, nil
	// NOTE: ESC + [0x40-0x5F] are C1 control codes.
	//       For the ones we don't recognize above, it might be more elegant to convert these to the appropriate rune
	//       (i.e., add 0x40 to make it 0x80-0x9F) and call handleCtrl instead of handleEsc.
	case c >= 0x40 && c <= 0x5f:
		p.handleCtrl(c + 0x40)
		return parseOutput, parserPaused
	// All other printable characters
	case c >= 0x30 && c <= 0x7E:
		p.handleEsc("", c)
		return parseOutput, parserPaused
	default:
		_ = p.buf.UnreadByte()
	}
	return parseEscape, nil
}

// parseEscapeIntermediate parses nF sequences
// https://en.wikipedia.org/wiki/ANSI_escape_code#nF_Escape_sequences
func parseEscapeIntermediate(p *parser) (state, error) {
	c, err := p.buf.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case c == ascii.DEL:
		// ignore
	case c >= 0x20 && c <= 0x2f:
		p.collectIntermediate(c)
	case c >= 0x30 && c <= 0x7e:
		p.handleEsc(p.intermediates(), c)
		return parseOutput, parserPaused
	default:
		_ = p.buf.UnreadByte()
	}
	return parseEscapeIntermediate, nil
}

func parseOSCString(p *parser) (state, error) {
	p.clear()
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case c == '\a':
			p.handleOSC(p.params())
			return parseOutput, parserPaused
		case ctrlCode(c):
			// ignore
		case graphicalCode(c):
			// NOTE: By collecting bytes here, we restrict the terminal's ability to handle certain large sequences,
			//       like files, until the whole string is read.
			p.collectParam(c)
		case c == ascii.ESC:
			// includes ST
			p.handleOSC(p.params())
			return parseEscape, parserPaused
		default: // should only happen on a terminating ctrl code
			p.handleOSC(p.params())
			_ = p.buf.UnreadByte()
			return parseOSCString, parserPaused
		}
	}
}

func parseCSIEntry(p *parser) (state, error) {
	p.clear()

	c, err := p.buf.ReadByte()
	if err != nil {
		return nil, err
	}

	switch {
	case c == ascii.DEL:
		// ignore
	case c >= 0x20 && c <= 0x2f:
		p.collectIntermediate(c)
		return parseCSIIntermediate, nil
	case c >= '0' && c <= '9' || c == ';':
		p.collectParam(c)
		return parseCSIParam, nil
	case c >= 0x3c && c <= 0x3f:
		p.collectIntermediate(c)
		return parseCSIParam, nil
	case c == 0x3a:
		return parseCSIIgnore, nil
	case c >= 0x40 && c <= 0x7e:
		p.handleCSI([]string{""}, "", c)
		return parseOutput, parserPaused
	default:
		_ = p.buf.UnreadByte()
	}
	return parseCSIEntry, nil
}

func parseCSIParam(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case c == ascii.DEL:
			// ignore
		case c >= 0x20 && c <= 0x2f:
			p.collectIntermediate(c)
			return parseCSIIntermediate, nil
		case c >= '0' && c <= '9' || c == ';':
			p.collectParam(c)
		case c == 0x3a || c >= 0x3c && c <= 0x3f:
			return parseCSIIgnore, nil
		case c >= 0x40 && c <= 0x7e:
			p.handleCSI(p.params(), p.intermediates(), c)
			return parseOutput, parserPaused
		default:
			_ = p.buf.UnreadByte()
			return parseCSIParam, nil
		}
	}
}

func parseCSIIntermediate(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case c == ascii.DEL:
			// ignore
		case c >= 0x20 && c <= 0x2f:
			p.collectIntermediate(c)
		case c >= 0x30 && c <= 0x3f:
			return parseCSIIgnore, nil
		case c >= 0x40 && c <= 0x7e:
			p.handleCSI(p.params(), p.intermediates(), c)
			return parseOutput, parserPaused
		default:
			_ = p.buf.UnreadByte()
			return parseCSIIntermediate, nil
		}
	}
}

func parseCSIIgnore(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		var low = c &^ 0x80

		switch {
		case low >= 0x20 && low <= 0x3f:
			// ignore
		case low == ascii.DEL:
			// ignore
		case low >= 0x40 && low <= 0x7e:
			return parseOutput, nil
		default:
			_ = p.buf.UnreadByte()
			return parseCSIIgnore, nil
		}
	}
}

func parseDCSEntry(p *parser) (state, error) {
	p.clear()

	c, err := p.buf.ReadByte()
	if err != nil {
		return nil, err
	}

	switch {
	case c == ascii.DEL:
		// ignore
	case ctrlCode(c):
		// ignore
	case c >= 0x20 && c <= 0x2f:
		p.collectIntermediate(c)
		return parseDCSIntermediate, nil

	case c >= '0' && c <= '9' || c == ';':
		p.collectParam(c)
		return parseDCSParam, nil
	case c >= 0x3c && c <= 0x3f:
		p.collectIntermediate(c)
		return parseDCSParam, nil
	case c == 0x3a:
		return parseIgnoreAll, nil

	case c >= 0x40 && c <= 0x7e:
		_ = p.buf.UnreadByte()
		return parseDCSPassthrough, nil

	default:
		_ = p.buf.UnreadByte()
	}
	return parseDCSEntry, nil
}

func parseDCSIntermediate(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case c == ascii.DEL:
			// ignore
		case ctrlCode(c):
			// ignore
		case c >= 0x20 && c <= 0x2f:
			p.collectIntermediate(c)
		case c >= 0x30 && c <= 0x3f:
			return parseIgnoreAll, nil
		case c >= 0x40 && c <= 0x7e:
			_ = p.buf.UnreadByte()
			return parseDCSPassthrough, nil
		default:
			_ = p.buf.UnreadByte()
			return parseEscapeIntermediate, nil
		}
	}
}

func parseDCSParam(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case c == ascii.DEL:
			// ignore
		case ctrlCode(c):
			// ignore
		case c >= 0x20 && c <= 0x2f:
			p.collectIntermediate(c)
			return parseDCSIntermediate, nil
		case c >= '0' && c <= '9' || c == ';':
			p.collectParam(c)
		case c == 0x3a || c >= 0x3c && c <= 0x3f:
			return parseIgnoreAll, nil
		case c >= 0x40 && c <= 0x7e:
			_ = p.buf.UnreadByte()
			return parseDCSPassthrough, nil
		default:
			_ = p.buf.UnreadByte()
			return parseDCSParam, nil
		}
	}
}

func parseDCSPassthrough(p *parser) (state, error) {
	c, err := p.buf.ReadByte()
	if err != nil {
		return nil, err
	}
	p.handleDSC(p.params(), p.intermediates(), c)

	// TODO: stream data after final byte to dispatchHandler
	log.Print("DCS parser ignores data trailing final byte")
	return parseIgnoreAll, nil
}

func parseIgnoreAll(p *parser) (state, error) {
	for {
		c, err := p.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		switch {
		case ctrlCode(c):
			// ignore
		case graphicalCode(c):
			// ignore
		default:
			_ = p.buf.UnreadByte()
			return parseIgnoreAll, nil
		}
	}
}

// clear implements the "clear" action.
func (p *parser) clear() {
	p.partialParams = nil
	p.partialParam.Reset()
	p.partialIntermediates.Reset()
}

// collectIntermediate implements the "collect" action.
func (p *parser) collectIntermediate(c byte) {
	p.partialIntermediates.WriteByte(c)
}

// collectParam implements the "param" action.
// Note that this is used for both CSI and OSC, and it takes any byte, not just ['0'-'9']
func (p *parser) collectParam(c byte) {
	if c == ';' {
		p.partialParams = append(p.partialParams, p.partialParam.String())
		p.partialParam.Reset()
	} else {
		p.partialParam.WriteByte(c)
	}
}

func (p *parser) intermediates() string {
	return p.partialIntermediates.String()
}

func (p *parser) params() []string {
	return append(p.partialParams, p.partialParam.String())
}
