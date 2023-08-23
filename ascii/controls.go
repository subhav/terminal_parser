package ascii

// ASCII C0 Control Characters
const (
	NUL byte = 0x00 // null
	SOH byte = 0x01 // start of heading
	STX byte = 0x02 // start of text
	ETX byte = 0x03 // end of text
	EOT byte = 0x04 // end of transmission
	ENQ byte = 0x05 // enquiry
	ACK byte = 0x06 // acknowledge
	BEL byte = 0x07 // bell
	BS  byte = 0x08 // backspace
	TAB byte = 0x09 // horizontal tab
	LF  byte = 0x0A // NL line feed, new line
	VT  byte = 0x0B // vertical tab
	FF  byte = 0x0C // NP form feed, new page
	CR  byte = 0x0D // carriage return
	SO  byte = 0x0E // shift out
	SI  byte = 0x0F // shift in
	DLE byte = 0x10 // data link escape
	DC1 byte = 0x11 // device control 1
	DC2 byte = 0x12 // device control 2
	DC3 byte = 0x13 // device control 3
	DC4 byte = 0x14 // device control 4
	NAK byte = 0x15 // negative acknowledge
	SYN byte = 0x16 // synchronous idle
	ETB byte = 0x17 // end of transmission block
	CAN byte = 0x18 // cancel
	EM  byte = 0x19 // end of medium
	SUB byte = 0x1A // substitute
	ESC byte = 0x1B // escape
	FS  byte = 0x1C // file separator
	GS  byte = 0x1D // group separator
	RS  byte = 0x1E // record separator
	US  byte = 0x1F // unit separator
	DEL byte = 0x7F // delete
)

// Unicode C1 Control Characters
const (
	PAD  byte = 0x80 // padding character
	HOP  byte = 0x81 // high octet preset
	BPH  byte = 0x82 // break permitted here
	NBH  byte = 0x83 // no break here
	IND  byte = 0x84 // index
	NEL  byte = 0x85 // next line
	SSA  byte = 0x86 // start of selected area
	ESA  byte = 0x87 // end of selected area
	HTS  byte = 0x88 // character (horizontal) tabulation set
	HTJ  byte = 0x89 // character (horizontal) tabulation with justification
	LTS  byte = 0x8A // line (vertical) tabulation set
	PLD  byte = 0x8B // partial line forward (down)
	PLU  byte = 0x8C // partial line backward (up)
	RI   byte = 0x8D // reverse line feed (index)
	SS2  byte = 0x8E // single-shift two
	SS3  byte = 0x8F // single-shift three
	DCS  byte = 0x90 // device control string
	PU1  byte = 0x91 // private use one
	PU2  byte = 0x92 // private use two
	STS  byte = 0x93 // set transmit state
	CCH  byte = 0x94 // cancel character
	MW   byte = 0x95 // message waiting
	SPA  byte = 0x96 // start of protected area
	EPA  byte = 0x97 // end of protected area
	SOS  byte = 0x98 // start of string
	SGCI byte = 0x99 // single graphic character introducer
	SCI  byte = 0x9A // single character introducer
	CSI  byte = 0x9B // control sequence introducer
	ST   byte = 0x9C // string terminator
	OSC  byte = 0x9D // operating system command
	PM   byte = 0x9E // private message
	APC  byte = 0x9F // application program command
)
