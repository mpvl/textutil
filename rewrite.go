// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"unicode/utf8"

	"golang.org/x/text/transform"
)

// A RewriterFunc wraps a function to implement a stateless Rewriter.
type RewriterFunc func(s State)

func (r RewriterFunc) Rewrite(s State) { r(s) }
func (r RewriterFunc) Reset()          {}

// A Rewriter rewrites the next segment of input text. The Transformer created
// by NewTransformer will repeatedly call the Rewrite method of this Rewriter
// until it cannot make further progress or if an error is encountered.
type Rewriter interface {
	// Rewrite rewrites an indivisible segment of input. If any error is
	// encountered, all reads and writes made within the same call to Rewrite
	// will be discarded.
	//
	// It will not be called with empty input.
	Rewrite(c State)

	// Reset implements the Reset method of tranform.Transformer.
	Reset()
}

// NewTransformer returns a Transformer that uses the given Rewriter to
// transform input. The Reset method of the returned Transformer will call the
// Reset method of the Rewriter if it defines one.
func NewTransformer(r Rewriter) Transformer {
	return Transformer{&rewriter{rewrite: r}}
}

// rewriter implements the Transformer interface as defined in
// go.text/transform.
type rewriter struct {
	rewrite Rewriter

	state state
}

func (t *rewriter) Reset() { t.rewrite.Reset() }

func (t *rewriter) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	t.state = state{dst: dst, spanState: spanState{src: src, atEOF: atEOF}}
	s := &t.state

	for s.pSrc < len(src) {
		if !atEOF && !utf8.FullRune(src[s.pSrc:]) {
			return nDst, nSrc, transform.ErrShortSrc
		}

		if t.rewrite.Rewrite(s); s.err != nil {
			return nDst, nSrc, s.err
		}
		// Checkpoint the progress.
		nDst, nSrc = s.pDst, s.pSrc
	}
	return nDst, nSrc, nil
}

func (t *rewriter) Span(src []byte, atEOF bool) (nSrc int, err error) {
	t.state.spanState = spanState{src: src, atEOF: atEOF}
	s := &t.state.spanState

	for s.pSrc < len(src) {
		if !atEOF && !utf8.FullRune(src[s.pSrc:]) {
			return nSrc, transform.ErrShortSrc
		}

		if t.rewrite.Rewrite(s); s.err != nil {
			return nSrc, s.err
		}
		// Checkpoint the progress.
		nSrc = s.pSrc
	}
	return nSrc, nil
}

// State track the progress of a Transformer created from a Rewriter.
type State interface {
	// ReadRune returns the next rune from the source and the number of bytes
	// consumed. It returns (RuneError, 1) for Invalid UTF-8 bytes. If the
	// source buffer is empty, it will return (RuneError, 0).
	ReadRune() (r rune, size int)

	// UnreadRune unreads the most recently read rune and makes it available for
	// a next call to Rewrite. Only one call to UnreadRune is allowed per
	// Rewrite.
	UnreadRune()

	// WriteBytes writes the given byte slice to the destination and reports
	// whether the write was successful.
	WriteBytes(b []byte) bool

	// WriteString writes the given string to the destination and reports
	// whether the write was successful.
	WriteString(s string) bool

	// WriteRune writes the given rune to the destination and reports whether
	// the write was successful.
	WriteRune(r rune) bool

	// Write implements io.Writer. The user is advised to use WriteBytes when
	// conformance to io.Writer is not needed.
	Write(b []byte) (n int, err error)

	// SetError reports invalid source bytes.
	SetError(err error)
}

// A spanState is passed to a Rewriter for reading from and writing to the source
// and destination buffers.
type spanState struct {
	err         error
	pDst, pSrc  int
	src         []byte
	atEOF       bool
	readPastEnd bool // Used for UnreadRune.
}

// SetError sets the error to be returned by Transform. The first error set will
// be returned. An error may also be set by one of State's methods.
//
// A Rewriter should not set ErrShortSrc or ErrShortDst itself. Use the Bytes
// and String functions defined in go.text/transform, instead of the
// corresponding methods of the Transformer returned by Rewrite, to detect
// special errors set by a Rewriter.
func (s *spanState) SetError(err error) {
	if s.err == nil {
		s.err = err
	}
}

func (s *spanState) ReadRune() (r rune, size int) {
	// TODO: ASCII fast path.
	r, size = utf8.DecodeRune(s.src[s.pSrc:])
	if r == utf8.RuneError && size <= 1 {
		s.readPastEnd = size == 0
		if !s.atEOF && !utf8.FullRune(s.src[s.pSrc:]) {
			s.SetError(transform.ErrShortSrc)
			return r, 0
		}
	}
	s.pSrc += size
	return
}

func (s *spanState) UnreadRune() {
	if s.readPastEnd {
		return
	}
	if s.pSrc == 0 {
		panic("Unread called without any prior input read.")
	}
	_, sz := utf8.DecodeLastRune(s.src[:s.pSrc])
	s.pSrc -= sz
	return
}

// Write implements io.Writer. It sets and returns ErrEndOfSpan if they written
// output is not equal to the input.
func (s *spanState) Write(b []byte) (n int, err error) {
	if max := len(s.src) - s.pDst; len(b) > max {
		b = b[:max]
	}
	for i, c := range s.src[s.pDst : s.pDst+len(b)] {
		if b[i] != c {
			if s.err == nil {
				s.pDst += i
				s.err = transform.ErrEndOfSpan
				return i, s.err
			}
		}
	}
	s.pDst += len(b)
	return len(b), nil
}

// Write implements io.Writer. It sets and returns ErrEndOfSpan if they written
// output is not equal to the input.
func (s *spanState) WriteBytes(b []byte) bool {
	if max := len(s.src) - s.pDst; len(b) > max {
		b = b[:max]
	}
	for i, c := range s.src[s.pDst : s.pDst+len(b)] {
		if b[i] != c {
			if s.err == nil {
				s.pDst += i
				s.err = transform.ErrEndOfSpan
				return true
			}
		}
	}
	s.pDst += len(b)
	return false
}

// WriteString writes the given string to the destination and returns whether
// the write was successful. It sets ErrShortDst if the write was unsuccessful.
func (s *spanState) WriteString(str string) bool {
	if max := len(s.src) - s.pDst; len(str) > max {
		str = str[:max]
	}
	for i, c := range s.src[s.pDst : s.pDst+len(str)] {
		if str[i] != c {
			if s.err == nil {
				s.pDst += i
				s.err = transform.ErrEndOfSpan
				return true
			}
		}
	}
	s.pDst += len(str)
	return false
}

// WriteRune writes the given rune to the destination and whether the write was
// successful. It sets ErrShortDst if the write was unsuccessful.
func (s *spanState) WriteRune(r rune) bool {
	// TODO: ASCII fast path and other optimizations.
	var b [utf8.UTFMax]byte
	sz := utf8.EncodeRune(b[:], r)
	n, _ := s.Write(b[:sz])
	return n == sz
}

// A state is passed to a Rewriter for reading from and writing to the source
// and destination buffers.
type state struct {
	spanState
	dst []byte
}

func (s *state) SetError(err error) {
	if s.err == nil {
		s.err = err
	}
}

func (s *state) Write(b []byte) (n int, err error) {
	if copy(s.dst[s.pDst:], b) != len(b) {
		s.SetError(transform.ErrShortDst)
		return 0, transform.ErrShortDst
	}
	s.pDst += len(b)
	return len(b), nil
}

func (s *state) WriteBytes(b []byte) bool {
	if copy(s.dst[s.pDst:], b) != len(b) {
		s.SetError(transform.ErrShortDst)
		return false
	}
	s.pDst += len(b)
	return true
}

func (s *state) WriteString(str string) bool {
	if copy(s.dst[s.pDst:], str) != len(str) {
		s.SetError(transform.ErrShortDst)
		return false
	}
	s.pDst += len(str)
	return true
}

func (s *state) WriteRune(r rune) bool {
	// TODO: ASCII fast path and other optimizations.
	var b [utf8.UTFMax]byte
	n := utf8.EncodeRune(b[:], r)
	if copy(s.dst[s.pDst:], b[:n]) != n {
		s.SetError(transform.ErrShortDst)
		return false
	}
	s.pDst += n
	return true
}
