// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textproto

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// A Reader implements convenience methods for reading requests
// or responses from a text protocol network connection.
type Reader struct {
	R   *bufio.Reader
	dot *dotReader
	buf []byte // a re-usable buffer for readContinuedLineSlice
}

// NewReader returns a new Reader reading from r.
//
// To avoid denial of service attacks, the provided bufio.Reader
// should be reading from an io.LimitReader or similar Reader to bound
// the size of responses.
func NewReader(r *bufio.Reader) *Reader {
	return &Reader{R: r}
}

// ReadLine reads a single line from r,
// eliding the final \n or \r\n from the returned string.
func (r *Reader) ReadLine() (string, error) {
	line, err := r.readLineSlice()
	return string(line), err
}

// ReadLineBytes is like ReadLine but returns a []byte instead of a string.
func (r *Reader) ReadLineBytes() ([]byte, error) {
	line, err := r.readLineSlice()
	if line != nil {
		line = bytes.Clone(line)
	}
	return line, err
}

func (r *Reader) readLineSlice() ([]byte, error) {
	r.closeDot()
	var line []byte
	for {
		l, more, err := r.R.ReadLine()
		if err != nil {
			return nil, err
		}
		// Avoid the copy if the first call produced a full line.
		if line == nil && !more {
			return l, nil
		}
		line = append(line, l...)
		if !more {
			break
		}
	}
	return line, nil
}

// ReadContinuedLine reads a possibly continued line from r,
// eliding the final trailing ASCII white space.
// Lines after the first are considered continuations if they
// begin with a space or tab character. In the returned data,
// continuation lines are separated from the previous line
// only by a single space: the newline and leading white space
// are removed.
//
// For example, consider this input:
//
//	Line 1
//	  continued...
//	Line 2
//
// The first call to ReadContinuedLine will return "Line 1 continued..."
// and the second will return "Line 2".
//
// Empty lines are never continued.
func (r *Reader) ReadContinuedLine() (string, error) {
	line, err := r.readContinuedLineSlice(noValidation)
	return string(line), err
}

// trim returns s with leading and trailing spaces and tabs removed.
// It does not assume Unicode or UTF-8.
func trim(s []byte) []byte {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	n := len(s)
	for n > i && (s[n-1] == ' ' || s[n-1] == '\t') {
		n--
	}
	return s[i:n]
}

// ReadContinuedLineBytes is like ReadContinuedLine but
// returns a []byte instead of a string.
func (r *Reader) ReadContinuedLineBytes() ([]byte, error) {
	line, err := r.readContinuedLineSlice(noValidation)
	if line != nil {
		line = bytes.Clone(line)
	}
	return line, err
}

// readContinuedLineSlice reads continued lines from the reader buffer,
// returning a byte slice with all lines. The validateFirstLine function
// is run on the first read line, and if it returns an error then this
// error is returned from readContinuedLineSlice.
func (r *Reader) readContinuedLineSlice(validateFirstLine func([]byte) error) ([]byte, error) {
	if validateFirstLine == nil {
		return nil, fmt.Errorf("missing validateFirstLine func")
	}

	// Read the first line.
	line, err := r.readLineSlice()
	if err != nil {
		return nil, err
	}
	if len(line) == 0 { // blank line - no continuation
		return line, nil
	}

	if err := validateFirstLine(line); err != nil {
		return nil, err
	}

	// Optimistically assume that we have started to buffer the next line
	// and it starts with an ASCII letter (the next header key), or a blank
	// line, so we can avoid copying that buffered data around in memory
	// and skipping over non-existent whitespace.
	if r.R.Buffered() > 1 {
		peek, _ := r.R.Peek(2)
		if len(peek) > 0 && (isASCIILetter(peek[0]) || peek[0] == '\n') ||
			len(peek) == 2 && peek[0] == '\r' && peek[1] == '\n' {
			return trim(line), nil
		}
	}

	// ReadByte or the next readLineSlice will flush the read buffer;
	// copy the slice into buf.
	r.buf = append(r.buf[:0], trim(line)...)

	// Read continuation lines.
	for r.skipSpace() > 0 {
		line, err := r.readLineSlice()
		if err != nil {
			break
		}
		r.buf = append(r.buf, ' ')
		r.buf = append(r.buf, trim(line)...)
	}
	return r.buf, nil
}

// skipSpace skips R over all spaces and returns the number of bytes skipped.
func (r *Reader) skipSpace() int {
	n := 0
	for {
		c, err := r.R.ReadByte()
		if err != nil {
			// Bufio will keep err until next read.
			break
		}
		if c != ' ' && c != '\t' {
			r.R.UnreadByte()
			break
		}
		n++
	}
	return n
}

func (r *Reader) readCodeLine(expectCode int) (code int, continued bool, message string, err error) {
	line, err := r.ReadLine()
	if err != nil {
		return
	}
	return parseCodeLine(line, expectCode)
}

func parseCodeLine(line string, expectCode int) (code int, continued bool, message string, err error) {
	if len(line) < 4 || line[3] != ' ' && line[3] != '-' {
		err = ProtocolError("short response: " + line)
		return
	}
	continued = line[3] == '-'
	code, err = strconv.Atoi(line[0:3])
	if err != nil || code < 100 {
		err = ProtocolError("invalid response code: " + line)
		return
	}
	message = line[4:]
	if 1 <= expectCode && expectCode < 10 && code/100 != expectCode ||
		10 <= expectCode && expectCode < 100 && code/10 != expectCode ||
		100 <= expectCode && expectCode < 1000 && code != expectCode {
		err = &Error{code, message}
	}
	return
}

// ReadCodeLine reads a response code line of the form
//
//	code message
//
// where code is a three-digit status code and the message
// extends to the rest of the line. An example of such a line is:
//
//	220 plan9.bell-labs.com ESMTP
//
// If the prefix of the status does not match the digits in expectCode,
// ReadCodeLine returns with err set to &Error{code, message}.
// For example, if expectCode is 31, an error will be returned if
// the status is not in the range [310,319].
//
// If the response is multi-line, ReadCodeLine returns an error.
//
// An expectCode <= 0 disables the check of the status code.
func (r *Reader) ReadCodeLine(expectCode int) (code int, message string, err error) {
	code, continued, message, err := r.readCodeLine(expectCode)
	if err == nil && continued {
		err = ProtocolError("unexpected multi-line response: " + message)
	}
	return
}

// ReadResponse reads a multi-line response of the form:
//
//	code-message line 1
//	code-message line 2
//	...
//	code message line n
//
// where code is a three-digit status code. The first line starts with the
// code and a hyphen. The response is terminated by a line that starts
// with the same code followed by a space. Each line in message is
// separated by a newline (\n).
//
// See page 36 of RFC 959 (https://www.ietf.org/rfc/rfc959.txt) for
// details of another form of response accepted:
//
//	code-message line 1
//	message line 2
//	...
//	code message line n
//
// If the prefix of the status does not match the digits in expectCode,
// ReadResponse returns with err set to &Error{code, message}.
// For example, if expectCode is 31, an error will be returned if
// the status is not in the range [310,319].
//
// An expectCode <= 0 disables the check of the status code.
func (r *Reader) ReadResponse(expectCode int) (code int, message string, err error) {
	code, continued, message, err := r.readCodeLine(expectCode)
	multi := continued
	for continued {
		line, err := r.ReadLine()
		if err != nil {
			return 0, "", err
		}

		var code2 int
		var moreMessage string
		code2, continued, moreMessage, err = parseCodeLine(line, 0)
		if err != nil || code2 != code {
			message += "\n" + strings.TrimRight(line, "\r\n")
			continued = true
			continue
		}
		message += "\n" + moreMessage
	}
	if err != nil && multi && message != "" {
		// replace one line error message with all lines (full message)
		err = &Error{code, message}
	}
	return
}

// DotReader returns a new Reader that satisfies Reads using the
// decoded text of a dot-encoded block read from r.
// The returned Reader is only valid until the next call
// to a method on r.
//
// Dot encoding is a common framing used for data blocks
// in text protocols such as SMTP.  The data consists of a sequence
// of lines, each of which ends in "\r\n".  The sequence itself
// ends at a line containing just a dot: ".\r\n".  Lines beginning
// with a dot are escaped with an additional dot to avoid
// looking like the end of the sequence.
//
// The decoded form returned by the Reader's Read method
// rewrites the "\r\n" line endings into the simpler "\n",
// removes leading dot escapes if present, and stops with error io.EOF
// after consuming (and discarding) the end-of-sequence line.
func (r *Reader) DotReader() io.Reader {
	r.closeDot()
	r.dot = &dotReader{r: r}
	return r.dot
}

type dotReader struct {
	r     *Reader
	state int
}

// Read satisfies reads by decoding dot-encoded data read from d.r.
func (d *dotReader) Read(b []byte) (n int, err error) {
	// Run data through a simple state machine to
	// elide leading dots, rewrite trailing \r\n into \n,
	// and detect ending .\r\n line.
	const (
		stateBeginLine = iota // beginning of line; initial state; must be zero
		stateDot              // read . at beginning of line
		stateDotCR            // read .\r at beginning of line
		stateCR               // read \r (possibly at end of line)
		stateData             // reading data in middle of line
		stateEOF              // reached .\r\n end marker line
	)
	br := d.r.R
	for n < len(b) && d.state != stateEOF {
		var c byte
		c, err = br.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			break
		}
		switch d.state {
		case stateBeginLine:
			if c == '.' {
				d.state = stateDot
				continue
			}
			if c == '\r' {
				d.state = stateCR
				continue
			}
			d.state = stateData

		case stateDot:
			if c == '\r' {
				d.state = stateDotCR
				continue
			}
			if c == '\n' {
				d.state = stateEOF
				continue
			}
			d.state = stateData

		case stateDotCR:
			if c == '\n' {
				d.state = stateEOF
				continue
			}
			// Not part of .\r\n.
			// Consume leading dot and emit saved \r.
			br.UnreadByte()
			c = '\r'
			d.state = stateData

		case stateCR:
			if c == '\n' {
				d.state = stateBeginLine
				break
			}
			// Not part of \r\n. Emit saved \r
			br.UnreadByte()
			c = '\r'
			d.state = stateData

		case stateData:
			if c == '\r' {
				d.state = stateCR
				continue
			}
			if c == '\n' {
				d.state = stateBeginLine
			}
		}
		b[n] = c
		n++
	}
	if err == nil && d.state == stateEOF {
		err = io.EOF
	}
	if err != nil && d.r.dot == d {
		d.r.dot = nil
	}
	return
}

// closeDot drains the current DotReader if any,
// making sure that it reads until the ending dot line.
func (r *Reader) closeDot() {
	if r.dot == nil {
		return
	}
	buf := make([]byte, 128)
	for r.dot != nil {
		// When Read reaches EOF or an error,
		// it will set r.dot == nil.
		r.dot.Read(buf)
	}
}

// ReadDotBytes reads a dot-encoding and returns the decoded data.
//
// See the documentation for the DotReader method for details about dot-encoding.
func (r *Reader) ReadDotBytes() ([]byte, error) {
	return io.ReadAll(r.DotReader())
}

// ReadDotLines reads a dot-encoding and returns a slice
// containing the decoded lines, with the final \r\n or \n elided from each.
//
// See the documentation for the DotReader method for details about dot-encoding.
func (r *Reader) ReadDotLines() ([]string, error) {
	// We could use ReadDotBytes and then Split it,
	// but reading a line at a time avoids needing a
	// large contiguous block of memory and is simpler.
	var v []string
	var err error
	for {
		var line string
		line, err = r.ReadLine()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			break
		}

		// Dot by itself marks end; otherwise cut one dot.
		if len(line) > 0 && line[0] == '.' {
			if len(line) == 1 {
				break
			}
			line = line[1:]
		}
		v = append(v, line)
	}
	return v, err
}

var colon = []byte(":")

// ReadMIMEHeader reads a MIME-style header from r.
// The header is a sequence of possibly continued Key: Value lines
// ending in a blank line.
// The returned map m maps CanonicalMIMEHeaderKey(key) to a
// sequence of values in the same order encountered in the input.
//
// For example, consider this input:
//
//	My-Key: Value 1
//	Long-Key: Even
//	       Longer Value
//	My-Key: Value 2
//
// Given that input, ReadMIMEHeader returns the map:
//
//	map[string][]string{
//		"My-Key": {"Value 1", "Value 2"},
//		"Long-Key": {"Even Longer Value"},
//	}
func (r *Reader) ReadMIMEHeader() (MIMEHeader, error) {
	return readMIMEHeader(r, math.MaxInt64, math.MaxInt64)
}

// readMIMEHeader is a version of ReadMIMEHeader which takes a limit on the header size.
// It is called by the mime/multipart package.
func readMIMEHeader(r *Reader, maxMemory, maxHeaders int64) (MIMEHeader, error) {
	// Avoid lots of small slice allocations later by allocating one
	// large one ahead of time which we'll cut up into smaller
	// slices. If this isn't big enough later, we allocate small ones.
	var strs []string
	hint := r.upcomingHeaderKeys()
	if hint > 0 {
		if hint > 1000 {
			hint = 1000 // set a cap to avoid overallocation
		}
		strs = make([]string, hint)
	}

	m := make(MIMEHeader, hint)

	// Account for 400 bytes of overhead for the MIMEHeader, plus 200 bytes per entry.
	// Benchmarking map creation as of go1.20, a one-entry MIMEHeader is 416 bytes and large
	// MIMEHeaders average about 200 bytes per entry.
	maxMemory -= 400
	const mapEntryOverhead = 200

	// The first line cannot start with a leading space.
	if buf, err := r.R.Peek(1); err == nil && (buf[0] == ' ' || buf[0] == '\t') {
		line, err := r.readLineSlice()
		if err != nil {
			return m, err
		}
		return m, ProtocolError("malformed MIME header initial line: " + string(line))
	}

	for {
		kv, err := r.readContinuedLineSlice(mustHaveFieldNameColon)
		if len(kv) == 0 {
			return m, err
		}

		// Key ends at first colon.
		k, v, ok := bytes.Cut(kv, colon)
		if !ok {
			return m, ProtocolError("malformed MIME header line: " + string(kv))
		}
		key, ok := canonicalMIMEHeaderKey(k)
		if !ok {
			return m, ProtocolError("malformed MIME header line: " + string(kv))
		}
		for _, c := range v {
			if !validHeaderValueByte(c) {
				return m, ProtocolError("malformed MIME header line: " + string(kv))
			}
		}

		// As per RFC 7230 field-name is a token, tokens consist of one or more chars.
		// We could return a ProtocolError here, but better to be liberal in what we
		// accept, so if we get an empty key, skip it.
		if key == "" {
			continue
		}

		maxHeaders--
		if maxHeaders < 0 {
			return nil, errors.New("message too large")
		}

		// Skip initial spaces in value.
		value := string(bytes.TrimLeft(v, " \t"))

		vv := m[key]
		if vv == nil {
			maxMemory -= int64(len(key))
			maxMemory -= mapEntryOverhead
		}
		maxMemory -= int64(len(value))
		if maxMemory < 0 {
			// TODO: This should be a distinguishable error (ErrMessageTooLarge)
			// to allow mime/multipart to detect it.
			return m, errors.New("message too large")
		}
		if vv == nil && len(strs) > 0 {
			// More than likely this will be a single-element key.
			// Most headers aren't multi-valued.
			// Set the capacity on strs[0] to 1, so any future append
			// won't extend the slice into the other strings.
			vv, strs = strs[:1:1], strs[1:]
			vv[0] = value
			m[key] = vv
		} else {
			m[key] = append(vv, value)
		}

		if err != nil {
			return m, err
		}
	}
}

// noValidation is a no-op validation func for readContinuedLineSlice
// that permits any lines.
func noValidation(_ []byte) error { return nil }

// mustHaveFieldNameColon ensures that, per RFC 7230, the
// field-name is on a single line, so the first line must
// contain a colon.
func mustHaveFieldNameColon(line []byte) error {
	if bytes.IndexByte(line, ':') < 0 {
		return ProtocolError(fmt.Sprintf("malformed MIME header: missing colon: %q", line))
	}
	return nil
}

var nl = []byte("\n")

// upcomingHeaderKeys returns an approximation of the number of keys
// that will be in this header. If it gets confused, it returns 0.
func (r *Reader) upcomingHeaderKeys() (n int) {
	// Try to determine the 'hint' size.
	r.R.Peek(1) // force a buffer load if empty
	s := r.R.Buffered()
	if s == 0 {
		return
	}
	peek, _ := r.R.Peek(s)
	for len(peek) > 0 && n < 1000 {
		var line []byte
		line, peek, _ = bytes.Cut(peek, nl)
		if len(line) == 0 || (len(line) == 1 && line[0] == '\r') {
			// Blank line separating headers from the body.
			break
		}
		if line[0] == ' ' || line[0] == '\t' {
			// Folded continuation of the previous line.
			continue
		}
		n++
	}
	return n
}

// CanonicalMIMEHeaderKey returns the canonical format of the
// MIME header key s. The canonicalization converts the first
// letter and any letter following a hyphen to upper case;
// the rest are converted to lowercase. For example, the
// canonical key for "accept-encoding" is "Accept-Encoding".
// MIME header keys are assumed to be ASCII only.
// If s contains a space or invalid header field bytes, it is
// returned without modifications.
func CanonicalMIMEHeaderKey(s string) string {
	// Quick check for canonical encoding.
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !validHeaderFieldByte(c) {
			return s
		}
		if upper && 'a' <= c && c <= 'z' {
			s, _ = canonicalMIMEHeaderKey(s)
			return s
		}
		if !upper && 'A' <= c && c <= 'Z' {
			s, _ = canonicalMIMEHeaderKey(s)
			return s
		}
		upper = c == '-'
	}
	return s
}

const toLower = 'a' - 'A'

// validHeaderFieldByte reports whether c is a valid byte in a header
// field name. RFC 7230 says:
//
//	header-field   = field-name ":" OWS field-value OWS
//	field-name     = token
//	tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//	        "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
//	token = 1*tchar
func validHeaderFieldByte(c byte) bool {
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero,
	// and this function will return false.
	const mask = 0 |
		(1<<(10)-1)<<'0' |
		(1<<(26)-1)<<'a' |
		(1<<(26)-1)<<'A' |
		1<<'!' |
		1<<'#' |
		1<<'$' |
		1<<'%' |
		1<<'&' |
		1<<'\'' |
		1<<'*' |
		1<<'+' |
		1<<'-' |
		1<<'.' |
		1<<'^' |
		1<<'_' |
		1<<'`' |
		1<<'|' |
		1<<'~'
	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}

// validHeaderValueByte reports whether c is a valid byte in a header
// field value. RFC 7230 says:
//
//	field-content  = field-vchar [ 1*( SP / HTAB ) field-vchar ]
//	field-vchar    = VCHAR / obs-text
//	obs-text       = %x80-FF
//
// RFC 5234 says:
//
//	HTAB           =  %x09
//	SP             =  %x20
//	VCHAR          =  %x21-7E
func validHeaderValueByte(c byte) bool {
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero.
	// Since this is the obs-text range, we invert the mask to
	// create a bitmap with 1s for disallowed bytes.
	const mask = 0 |
		(1<<(0x7f-0x21)-1)<<0x21 | // VCHAR: %x21-7E
		1<<0x20 | // SP: %x20
		1<<0x09 // HTAB: %x09
	return ((uint64(1)<<c)&^(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&^(mask>>64)) == 0
}

// canonicalMIMEHeaderKey is like CanonicalMIMEHeaderKey but is
// allowed to mutate the provided byte slice before returning the
// string.
//
// For invalid inputs (if a contains spaces or non-token bytes), a
// is unchanged and a string copy is returned.
//
// ok is true if the header key contains only valid characters and spaces.
// ReadMIMEHeader accepts header keys containing spaces, but does not
// canonicalize them.
func canonicalMIMEHeaderKey[T interface{ []byte | string }](k T) (_ string, ok bool) {
	a := []byte(k)

	// See if a looks like a header key. If not, return it unchanged.
	noCanon := false
	for _, c := range a {
		if validHeaderFieldByte(c) {
			continue
		}
		// Don't canonicalize.
		if c == ' ' {
			// We accept invalid headers with a space before the
			// colon, but must not canonicalize them.
			// See https://go.dev/issue/34540.
			noCanon = true
			continue
		}
		return string(a), false
	}
	if noCanon {
		return string(a), true
	}

	upper := true
	for i, c := range a {
		// Canonicalize: first letter upper case
		// and upper case after each dash.
		// (Host, User-Agent, If-Modified-Since).
		// MIME headers are ASCII only, so no Unicode issues.
		if upper && 'a' <= c && c <= 'z' {
			c -= toLower
		} else if !upper && 'A' <= c && c <= 'Z' {
			c += toLower
		}
		a[i] = c
		upper = c == '-' // for next time
	}

	s := headerToString(a)
	if s == "" {
		s = string(a)
	}
	return s, true
}

// headerToString matches the canonical header name in the passed slice
// and returns the singleton string instance of known, common headers.
// If the header is not known, the passed slice is returned as a string.
//
// A header is eligible for inclusion in headerToString if, either:
//  1. It is a standard header, e.g. listed in a non-obsolete RFC or other
//     standards.
//  2. It is a header that can be shown to be commonly used in the wild.
func headerToString(a []byte) (s string) {
	switch string(a) { // Compiler knows how to avoid allocating the string.
	case "A-Im":
		s = "A-Im"
	case "Accept":
		s = "Accept"
	case "Accept-Additions":
		s = "Accept-Additions"
	case "Accept-Ch":
		s = "Accept-Ch"
	case "Accept-Charset":
		s = "Accept-Charset"
	case "Accept-Datetime":
		s = "Accept-Datetime"
	case "Accept-Encoding":
		s = "Accept-Encoding"
	case "Accept-Features":
		s = "Accept-Features"
	case "Accept-Language":
		s = "Accept-Language"
	case "Accept-Patch":
		s = "Accept-Patch"
	case "Accept-Post":
		s = "Accept-Post"
	case "Accept-Ranges":
		s = "Accept-Ranges"
	case "Access-Control":
		s = "Access-Control"
	case "Access-Control-Allow-Credentials":
		s = "Access-Control-Allow-Credentials"
	case "Access-Control-Allow-Headers":
		s = "Access-Control-Allow-Headers"
	case "Access-Control-Allow-Methods":
		s = "Access-Control-Allow-Methods"
	case "Access-Control-Allow-Origin":
		s = "Access-Control-Allow-Origin"
	case "Access-Control-Expose-Headers":
		s = "Access-Control-Expose-Headers"
	case "Access-Control-Max-Age":
		s = "Access-Control-Max-Age"
	case "Access-Control-Request-Headers":
		s = "Access-Control-Request-Headers"
	case "Access-Control-Request-Method":
		s = "Access-Control-Request-Method"
	case "Age":
		s = "Age"
	case "Allow":
		s = "Allow"
	case "Alpn":
		s = "Alpn"
	case "Also-Control":
		s = "Also-Control"
	case "Alt-Svc":
		s = "Alt-Svc"
	case "Alt-Used":
		s = "Alt-Used"
	case "Alternate-Recipient":
		s = "Alternate-Recipient"
	case "Alternates":
		s = "Alternates"
	case "Amp-Cache-Transform":
		s = "Amp-Cache-Transform"
	case "Apply-To-Redirect-Ref":
		s = "Apply-To-Redirect-Ref"
	case "Approved":
		s = "Approved"
	case "Arc-Authentication-Results":
		s = "Arc-Authentication-Results"
	case "Arc-Message-Signature":
		s = "Arc-Message-Signature"
	case "Arc-Seal":
		s = "Arc-Seal"
	case "Archive":
		s = "Archive"
	case "Archived-At":
		s = "Archived-At"
	case "Article-Names":
		s = "Article-Names"
	case "Article-Updates":
		s = "Article-Updates"
	case "Authentication-Control":
		s = "Authentication-Control"
	case "Authentication-Info":
		s = "Authentication-Info"
	case "Authentication-Results":
		s = "Authentication-Results"
	case "Authorization":
		s = "Authorization"
	case "Auto-Submitted":
		s = "Auto-Submitted"
	case "Autoforwarded":
		s = "Autoforwarded"
	case "Autosubmitted":
		s = "Autosubmitted"
	case "Base":
		s = "Base"
	case "Bcc":
		s = "Bcc"
	case "Body":
		s = "Body"
	case "C-Ext":
		s = "C-Ext"
	case "C-Man":
		s = "C-Man"
	case "C-Opt":
		s = "C-Opt"
	case "C-Pep":
		s = "C-Pep"
	case "C-Pep-Info":
		s = "C-Pep-Info"
	case "Cache-Control":
		s = "Cache-Control"
	case "Cache-Status":
		s = "Cache-Status"
	case "Cal-Managed-Id":
		s = "Cal-Managed-Id"
	case "Caldav-Timezones":
		s = "Caldav-Timezones"
	case "Cancel-Key":
		s = "Cancel-Key"
	case "Cancel-Lock":
		s = "Cancel-Lock"
	case "Capsule-Protocol":
		s = "Capsule-Protocol"
	case "Cc":
		s = "Cc"
	case "Cdn-Cache-Control":
		s = "Cdn-Cache-Control"
	case "Cdn-Loop":
		s = "Cdn-Loop"
	case "Cert-Not-After":
		s = "Cert-Not-After"
	case "Cert-Not-Before":
		s = "Cert-Not-Before"
	case "Clear-Site-Data":
		s = "Clear-Site-Data"
	case "Client-Cert":
		s = "Client-Cert"
	case "Client-Cert-Chain":
		s = "Client-Cert-Chain"
	case "Close":
		s = "Close"
	case "Comments":
		s = "Comments"
	case "Configuration-Context":
		s = "Configuration-Context"
	case "Connection":
		s = "Connection"
	case "Content-Alternative":
		s = "Content-Alternative"
	case "Content-Base":
		s = "Content-Base"
	case "Content-Description":
		s = "Content-Description"
	case "Content-Disposition":
		s = "Content-Disposition"
	case "Content-Duration":
		s = "Content-Duration"
	case "Content-Encoding":
		s = "Content-Encoding"
	case "Content-Features":
		s = "Content-Features"
	case "Content-Id":
		s = "Content-Id"
	case "Content-Identifier":
		s = "Content-Identifier"
	case "Content-Language":
		s = "Content-Language"
	case "Content-Length":
		s = "Content-Length"
	case "Content-Location":
		s = "Content-Location"
	case "Content-Md5":
		s = "Content-Md5"
	case "Content-Range":
		s = "Content-Range"
	case "Content-Return":
		s = "Content-Return"
	case "Content-Script-Type":
		s = "Content-Script-Type"
	case "Content-Security-Policy":
		s = "Content-Security-Policy"
	case "Content-Security-Policy-Report-Only":
		s = "Content-Security-Policy-Report-Only"
	case "Content-Style-Type":
		s = "Content-Style-Type"
	case "Content-Transfer-Encoding":
		s = "Content-Transfer-Encoding"
	case "Content-Translation-Type":
		s = "Content-Translation-Type"
	case "Content-Type":
		s = "Content-Type"
	case "Content-Version":
		s = "Content-Version"
	case "Control":
		s = "Control"
	case "Conversion":
		s = "Conversion"
	case "Conversion-With-Loss":
		s = "Conversion-With-Loss"
	case "Cookie":
		s = "Cookie"
	case "Cookie2":
		s = "Cookie2"
	case "Cross-Origin-Embedder-Policy":
		s = "Cross-Origin-Embedder-Policy"
	case "Cross-Origin-Embedder-Policy-Report-Only":
		s = "Cross-Origin-Embedder-Policy-Report-Only"
	case "Cross-Origin-Opener-Policy":
		s = "Cross-Origin-Opener-Policy"
	case "Cross-Origin-Opener-Policy-Report-Only":
		s = "Cross-Origin-Opener-Policy-Report-Only"
	case "Cross-Origin-Resource-Policy":
		s = "Cross-Origin-Resource-Policy"
	case "Dasl":
		s = "Dasl"
	case "Date":
		s = "Date"
	case "Date-Received":
		s = "Date-Received"
	case "Dav":
		s = "Dav"
	case "Default-Style":
		s = "Default-Style"
	case "Deferred-Delivery":
		s = "Deferred-Delivery"
	case "Delivery-Date":
		s = "Delivery-Date"
	case "Delta-Base":
		s = "Delta-Base"
	case "Depth":
		s = "Depth"
	case "Derived-From":
		s = "Derived-From"
	case "Destination":
		s = "Destination"
	case "Differential-Id":
		s = "Differential-Id"
	case "Digest":
		s = "Digest"
	case "Discarded-X400-Ipms-Extensions":
		s = "Discarded-X400-Ipms-Extensions"
	case "Discarded-X400-Mts-Extensions":
		s = "Discarded-X400-Mts-Extensions"
	case "Disclose-Recipients":
		s = "Disclose-Recipients"
	case "Disposition-Notification-Options":
		s = "Disposition-Notification-Options"
	case "Disposition-Notification-To":
		s = "Disposition-Notification-To"
	case "Distribution":
		s = "Distribution"
	case "Dkim-Signature":
		s = "Dkim-Signature"
	case "Dl-Expansion-History":
		s = "Dl-Expansion-History"
	case "Dnt":
		s = "Dnt"
	case "Downgraded-Bcc":
		s = "Downgraded-Bcc"
	case "Downgraded-Cc":
		s = "Downgraded-Cc"
	case "Downgraded-Disposition-Notification-To":
		s = "Downgraded-Disposition-Notification-To"
	case "Downgraded-Final-Recipient":
		s = "Downgraded-Final-Recipient"
	case "Downgraded-From":
		s = "Downgraded-From"
	case "Downgraded-In-Reply-To":
		s = "Downgraded-In-Reply-To"
	case "Downgraded-Mail-From":
		s = "Downgraded-Mail-From"
	case "Downgraded-Message-Id":
		s = "Downgraded-Message-Id"
	case "Downgraded-Original-Recipient":
		s = "Downgraded-Original-Recipient"
	case "Downgraded-Rcpt-To":
		s = "Downgraded-Rcpt-To"
	case "Downgraded-References":
		s = "Downgraded-References"
	case "Downgraded-Reply-To":
		s = "Downgraded-Reply-To"
	case "Downgraded-Resent-Bcc":
		s = "Downgraded-Resent-Bcc"
	case "Downgraded-Resent-Cc":
		s = "Downgraded-Resent-Cc"
	case "Downgraded-Resent-From":
		s = "Downgraded-Resent-From"
	case "Downgraded-Resent-Reply-To":
		s = "Downgraded-Resent-Reply-To"
	case "Downgraded-Resent-Sender":
		s = "Downgraded-Resent-Sender"
	case "Downgraded-Resent-To":
		s = "Downgraded-Resent-To"
	case "Downgraded-Return-Path":
		s = "Downgraded-Return-Path"
	case "Downgraded-Sender":
		s = "Downgraded-Sender"
	case "Downgraded-To":
		s = "Downgraded-To"
	case "Early-Data":
		s = "Early-Data"
	case "Ediint-Features":
		s = "Ediint-Features"
	case "Encoding":
		s = "Encoding"
	case "Encrypted":
		s = "Encrypted"
	case "Etag":
		s = "Etag"
	case "Expect":
		s = "Expect"
	case "Expect-Ct":
		s = "Expect-Ct"
	case "Expires":
		s = "Expires"
	case "Expiry-Date":
		s = "Expiry-Date"
	case "Ext":
		s = "Ext"
	case "Followup-To":
		s = "Followup-To"
	case "Forwarded":
		s = "Forwarded"
	case "From":
		s = "From"
	case "Generate-Delivery-Report":
		s = "Generate-Delivery-Report"
	case "Getprofile":
		s = "Getprofile"
	case "Hobareg":
		s = "Hobareg"
	case "Host":
		s = "Host"
	case "Http2-Settings":
		s = "Http2-Settings"
	case "If":
		s = "If"
	case "If-Match":
		s = "If-Match"
	case "If-Modified-Since":
		s = "If-Modified-Since"
	case "If-None-Match":
		s = "If-None-Match"
	case "If-Range":
		s = "If-Range"
	case "If-Schedule-Tag-Match":
		s = "If-Schedule-Tag-Match"
	case "If-Unmodified-Since":
		s = "If-Unmodified-Since"
	case "Im":
		s = "Im"
	case "Importance":
		s = "Importance"
	case "In-Reply-To":
		s = "In-Reply-To"
	case "Include-Referred-Token-Binding-Id":
		s = "Include-Referred-Token-Binding-Id"
	case "Incomplete-Copy":
		s = "Incomplete-Copy"
	case "Injection-Date":
		s = "Injection-Date"
	case "Injection-Info":
		s = "Injection-Info"
	case "Isolation":
		s = "Isolation"
	case "Keep-Alive":
		s = "Keep-Alive"
	case "Keywords":
		s = "Keywords"
	case "Label":
		s = "Label"
	case "Language":
		s = "Language"
	case "Last-Event-Id":
		s = "Last-Event-Id"
	case "Last-Modified":
		s = "Last-Modified"
	case "Latest-Delivery-Time":
		s = "Latest-Delivery-Time"
	case "Lines":
		s = "Lines"
	case "Link":
		s = "Link"
	case "List-Archive":
		s = "List-Archive"
	case "List-Help":
		s = "List-Help"
	case "List-Id":
		s = "List-Id"
	case "List-Owner":
		s = "List-Owner"
	case "List-Post":
		s = "List-Post"
	case "List-Subscribe":
		s = "List-Subscribe"
	case "List-Unsubscribe":
		s = "List-Unsubscribe"
	case "List-Unsubscribe-Post":
		s = "List-Unsubscribe-Post"
	case "Location":
		s = "Location"
	case "Lock-Token":
		s = "Lock-Token"
	case "Man":
		s = "Man"
	case "Max-Forwards":
		s = "Max-Forwards"
	case "Memento-Datetime":
		s = "Memento-Datetime"
	case "Message-Context":
		s = "Message-Context"
	case "Message-Id":
		s = "Message-Id"
	case "Message-Type":
		s = "Message-Type"
	case "Meter":
		s = "Meter"
	case "Method-Check":
		s = "Method-Check"
	case "Method-Check-Expires":
		s = "Method-Check-Expires"
	case "Mime-Version":
		s = "Mime-Version"
	case "Mmhs-Acp127-Message-Identifier":
		s = "Mmhs-Acp127-Message-Identifier"
	case "Mmhs-Codress-Message-Indicator":
		s = "Mmhs-Codress-Message-Indicator"
	case "Mmhs-Copy-Precedence":
		s = "Mmhs-Copy-Precedence"
	case "Mmhs-Exempted-Address":
		s = "Mmhs-Exempted-Address"
	case "Mmhs-Extended-Authorisation-Info":
		s = "Mmhs-Extended-Authorisation-Info"
	case "Mmhs-Handling-Instructions":
		s = "Mmhs-Handling-Instructions"
	case "Mmhs-Message-Instructions":
		s = "Mmhs-Message-Instructions"
	case "Mmhs-Message-Type":
		s = "Mmhs-Message-Type"
	case "Mmhs-Originator-Plad":
		s = "Mmhs-Originator-Plad"
	case "Mmhs-Originator-Reference":
		s = "Mmhs-Originator-Reference"
	case "Mmhs-Other-Recipients-Indicator-Cc":
		s = "Mmhs-Other-Recipients-Indicator-Cc"
	case "Mmhs-Other-Recipients-Indicator-To":
		s = "Mmhs-Other-Recipients-Indicator-To"
	case "Mmhs-Primary-Precedence":
		s = "Mmhs-Primary-Precedence"
	case "Mmhs-Subject-Indicator-Codes":
		s = "Mmhs-Subject-Indicator-Codes"
	case "Mt-Priority":
		s = "Mt-Priority"
	case "Negotiate":
		s = "Negotiate"
	case "Newsgroups":
		s = "Newsgroups"
	case "Nntp-Posting-Date":
		s = "Nntp-Posting-Date"
	case "Nntp-Posting-Host":
		s = "Nntp-Posting-Host"
	case "Obsoletes":
		s = "Obsoletes"
	case "Odata-Entityid":
		s = "Odata-Entityid"
	case "Odata-Isolation":
		s = "Odata-Isolation"
	case "Odata-Maxversion":
		s = "Odata-Maxversion"
	case "Odata-Version":
		s = "Odata-Version"
	case "Opt":
		s = "Opt"
	case "Optional-Www-Authenticate":
		s = "Optional-Www-Authenticate"
	case "Ordering-Type":
		s = "Ordering-Type"
	case "Organization":
		s = "Organization"
	case "Origin":
		s = "Origin"
	case "Origin-Agent-Cluster":
		s = "Origin-Agent-Cluster"
	case "Original-Encoded-Information-Types":
		s = "Original-Encoded-Information-Types"
	case "Original-From":
		s = "Original-From"
	case "Original-Message-Id":
		s = "Original-Message-Id"
	case "Original-Recipient":
		s = "Original-Recipient"
	case "Original-Sender":
		s = "Original-Sender"
	case "Original-Subject":
		s = "Original-Subject"
	case "Originator-Return-Address":
		s = "Originator-Return-Address"
	case "Oscore":
		s = "Oscore"
	case "Oslc-Core-Version":
		s = "Oslc-Core-Version"
	case "Overwrite":
		s = "Overwrite"
	case "P3p":
		s = "P3p"
	case "Path":
		s = "Path"
	case "Pep":
		s = "Pep"
	case "Pep-Info":
		s = "Pep-Info"
	case "Pics-Label":
		s = "Pics-Label"
	case "Ping-From":
		s = "Ping-From"
	case "Ping-To":
		s = "Ping-To"
	case "Position":
		s = "Position"
	case "Posting-Version":
		s = "Posting-Version"
	case "Pragma":
		s = "Pragma"
	case "Prefer":
		s = "Prefer"
	case "Preference-Applied":
		s = "Preference-Applied"
	case "Prevent-Nondelivery-Report":
		s = "Prevent-Nondelivery-Report"
	case "Priority":
		s = "Priority"
	case "Profileobject":
		s = "Profileobject"
	case "Protocol":
		s = "Protocol"
	case "Protocol-Info":
		s = "Protocol-Info"
	case "Protocol-Query":
		s = "Protocol-Query"
	case "Protocol-Request":
		s = "Protocol-Request"
	case "Proxy-Authenticate":
		s = "Proxy-Authenticate"
	case "Proxy-Authentication-Info":
		s = "Proxy-Authentication-Info"
	case "Proxy-Authorization":
		s = "Proxy-Authorization"
	case "Proxy-Features":
		s = "Proxy-Features"
	case "Proxy-Instruction":
		s = "Proxy-Instruction"
	case "Proxy-Status":
		s = "Proxy-Status"
	case "Public":
		s = "Public"
	case "Public-Key-Pins":
		s = "Public-Key-Pins"
	case "Public-Key-Pins-Report-Only":
		s = "Public-Key-Pins-Report-Only"
	case "Range":
		s = "Range"
	case "Received":
		s = "Received"
	case "Received-Spf":
		s = "Received-Spf"
	case "Redirect-Ref":
		s = "Redirect-Ref"
	case "References":
		s = "References"
	case "Referer":
		s = "Referer"
	case "Referer-Root":
		s = "Referer-Root"
	case "Refresh":
		s = "Refresh"
	case "Relay-Version":
		s = "Relay-Version"
	case "Repeatability-Client-Id":
		s = "Repeatability-Client-Id"
	case "Repeatability-First-Sent":
		s = "Repeatability-First-Sent"
	case "Repeatability-Request-Id":
		s = "Repeatability-Request-Id"
	case "Repeatability-Result":
		s = "Repeatability-Result"
	case "Replay-Nonce":
		s = "Replay-Nonce"
	case "Reply-By":
		s = "Reply-By"
	case "Reply-To":
		s = "Reply-To"
	case "Require-Recipient-Valid-Since":
		s = "Require-Recipient-Valid-Since"
	case "Resent-Bcc":
		s = "Resent-Bcc"
	case "Resent-Cc":
		s = "Resent-Cc"
	case "Resent-Date":
		s = "Resent-Date"
	case "Resent-From":
		s = "Resent-From"
	case "Resent-Message-Id":
		s = "Resent-Message-Id"
	case "Resent-Reply-To":
		s = "Resent-Reply-To"
	case "Resent-Sender":
		s = "Resent-Sender"
	case "Resent-To":
		s = "Resent-To"
	case "Retry-After":
		s = "Retry-After"
	case "Return-Path":
		s = "Return-Path"
	case "Safe":
		s = "Safe"
	case "Schedule-Reply":
		s = "Schedule-Reply"
	case "Schedule-Tag":
		s = "Schedule-Tag"
	case "Sec-Gpc":
		s = "Sec-Gpc"
	case "Sec-Purpose":
		s = "Sec-Purpose"
	case "Sec-Token-Binding":
		s = "Sec-Token-Binding"
	case "Sec-Websocket-Accept":
		s = "Sec-Websocket-Accept"
	case "Sec-Websocket-Extensions":
		s = "Sec-Websocket-Extensions"
	case "Sec-Websocket-Key":
		s = "Sec-Websocket-Key"
	case "Sec-Websocket-Protocol":
		s = "Sec-Websocket-Protocol"
	case "Sec-Websocket-Version":
		s = "Sec-Websocket-Version"
	case "Security-Scheme":
		s = "Security-Scheme"
	case "See-Also":
		s = "See-Also"
	case "Sender":
		s = "Sender"
	case "Sensitivity":
		s = "Sensitivity"
	case "Server":
		s = "Server"
	case "Server-Timing":
		s = "Server-Timing"
	case "Set-Cookie":
		s = "Set-Cookie"
	case "Set-Cookie2":
		s = "Set-Cookie2"
	case "Setprofile":
		s = "Setprofile"
	case "Slug":
		s = "Slug"
	case "Soapaction":
		s = "Soapaction"
	case "Solicitation":
		s = "Solicitation"
	case "Status-Uri":
		s = "Status-Uri"
	case "Strict-Transport-Security":
		s = "Strict-Transport-Security"
	case "Subect":
		s = "Subect"
	case "Subject":
		s = "Subject"
	case "Summary":
		s = "Summary"
	case "Sunset":
		s = "Sunset"
	case "Supersedes":
		s = "Supersedes"
	case "Surrogate-Capability":
		s = "Surrogate-Capability"
	case "Surrogate-Control":
		s = "Surrogate-Control"
	case "Tcn":
		s = "Tcn"
	case "Te":
		s = "Te"
	case "Timeout":
		s = "Timeout"
	case "Timing-Allow-Origin":
		s = "Timing-Allow-Origin"
	case "Tk":
		s = "Tk"
	case "Tls-Report-Domain":
		s = "Tls-Report-Domain"
	case "Tls-Report-Submitter":
		s = "Tls-Report-Submitter"
	case "Tls-Required":
		s = "Tls-Required"
	case "To":
		s = "To"
	case "Topic":
		s = "Topic"
	case "Traceparent":
		s = "Traceparent"
	case "Tracestate":
		s = "Tracestate"
	case "Trailer":
		s = "Trailer"
	case "Transfer-Encoding":
		s = "Transfer-Encoding"
	case "Ttl":
		s = "Ttl"
	case "Upgrade":
		s = "Upgrade"
	case "Upgrade-Insecure-Requests":
		s = "Upgrade-Insecure-Requests"
	case "Urgency":
		s = "Urgency"
	case "Uri":
		s = "Uri"
	case "User-Agent":
		s = "User-Agent"
	case "Variant-Vary":
		s = "Variant-Vary"
	case "Vary":
		s = "Vary"
	case "Vbr-Info":
		s = "Vbr-Info"
	case "Via":
		s = "Via"
	case "Want-Digest":
		s = "Want-Digest"
	case "Warning":
		s = "Warning"
	case "Www-Authenticate":
		s = "Www-Authenticate"
	case "X-B3-Spanid":
		s = "X-B3-Spanid"
	case "X-B3-Traceid":
		s = "X-B3-Traceid"
	case "X-Content-Type-Options":
		s = "X-Content-Type-Options"
	case "X-Correlation-Id":
		s = "X-Correlation-Id"
	case "X-Forwarded-For":
		s = "X-Forwarded-For"
	case "X-Forwarded-Host":
		s = "X-Forwarded-Host"
	case "X-Forwarded-Proto":
		s = "X-Forwarded-Proto"
	case "X-Frame-Options":
		s = "X-Frame-Options"
	case "X-Imforwards":
		s = "X-Imforwards"
	case "X-Powered-By":
		s = "X-Powered-By"
	case "X-Real-Ip":
		s = "X-Real-Ip"
	case "X-Request-Id":
		s = "X-Request-Id"
	case "X-Requested-With":
		s = "X-Requested-With"
	case "X400-Content-Identifier":
		s = "X400-Content-Identifier"
	case "X400-Content-Return":
		s = "X400-Content-Return"
	case "X400-Content-Type":
		s = "X400-Content-Type"
	case "X400-Mts-Identifier":
		s = "X400-Mts-Identifier"
	case "X400-Originator":
		s = "X400-Originator"
	case "X400-Received":
		s = "X400-Received"
	case "X400-Recipients":
		s = "X400-Recipients"
	case "X400-Trace":
		s = "X400-Trace"
	case "Xref":
		s = "Xref"
	}
	return
}
