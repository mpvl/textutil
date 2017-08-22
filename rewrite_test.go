// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"errors"
	"testing"

	"golang.org/x/text/transform"
)

// rwCopy copies input verbatim.
type rwCopy struct{}

func (rwCopy) Reset() {}

func (rwCopy) Rewrite(s State) {
	r, _ := s.ReadRune()
	s.WriteRune(r)
}

// rwReplaceAll rewrites all incoming runes to 'a'.
type rwReplaceAll struct{}

func (rwReplaceAll) Reset() {}

func (rwReplaceAll) Rewrite(s State) {
	_, _ = s.ReadRune()
	s.WriteRune('a')
}

type genericRewriter func(State)

func (r genericRewriter) Rewrite(s State) {
	r(s)
}

func (r genericRewriter) Reset() {}

func rw(r genericRewriter) transform.SpanningTransformer {
	return NewTransformer(r)
}

// rwLast writes the success of the last call to Rewrite as the first rune.
func rwLast(f func(State) bool) transform.SpanningTransformer {
	var last bool
	return rw(func(s State) {
		if last {
			s.WriteRune('T')
		} else {
			s.WriteRune('F')
		}
		last = f(s)
	})
}

func TestRewriteMain(t *testing.T) {
	var myError = errors.New("my error")

	const large = 1000

	testCases := []transformTest{{
		desc:  "Don't call with empty input.",
		szDst: large,
		atEOF: true,
		in:    "",
		t:     rw(func(s State) { t.Error("0: should not reach here") }),
	}, {
		desc:    "Don't call more than once.",
		szDst:   large,
		atEOF:   true,
		in:      "1",
		out:     "a",
		outFull: "a",
		t:       NewTransformer(rwReplaceAll{}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Don't call more than twice.",
		szDst:   large,
		atEOF:   true,
		in:      "11",
		out:     "aa",
		outFull: "aa",
		t:       NewTransformer(rwReplaceAll{}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Don't call for incomplete UTF-8.",
		szDst:   large,
		atEOF:   false,
		in:      "e\xcc",
		out:     "a",
		outFull: "aa",
		err:     transform.ErrShortSrc,
		t:       NewTransformer(rwReplaceAll{}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Call for incomplete UTF-8 at end of input.",
		szDst:   large,
		atEOF:   true,
		in:      "e\xcc",
		out:     "e\ufffd",
		outFull: "e\ufffd",
		t:       NewTransformer(rwCopy{}),
		errSpan: transform.ErrEndOfSpan,
		nSpan:   1,
	}, {
		desc:    "Call for known illegal bytes.",
		szDst:   large,
		atEOF:   false,
		in:      "e\x80",
		out:     "e\ufffd",
		outFull: "e\ufffd",
		t:       NewTransformer(rwCopy{}),
		errSpan: transform.ErrEndOfSpan,
		nSpan:   1,
	}, {
		desc:    "Discard all if we get an ErrShortSrc.",
		szDst:   large,
		atEOF:   false,
		in:      "aaa",
		outFull: "xxx",
		err:     transform.ErrShortSrc,
		t: rw(func(s State) {
			for {
				if _, size := s.ReadRune(); size == 0 {
					return
				}
				s.WriteString("x")
			}
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Discard all if we get an ErrShortDst.",
		szDst:   1,
		atEOF:   false,
		in:      "aaa",
		outFull: "xxx",
		err:     transform.ErrShortDst,
		t: rw(func(s State) {
			for {
				if _, size := s.ReadRune(); size == 0 {
					return
				}
				s.WriteString("x")
			}
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Discard last if we get an ErrShortDst.",
		szDst:   1,
		atEOF:   false,
		in:      "aaa",
		out:     "a",
		outFull: "aaa",
		err:     transform.ErrShortDst,
		t:       NewTransformer(&rwCopy{}),
		nSpan:   3,
	}, {
		desc:    "Unread.",
		szDst:   large,
		atEOF:   true,
		in:      "a\u0300\u2208\U0001030fx",
		out:     "a\u0300\u2208\U0001030fx",
		outFull: "a\u0300\u2208\U0001030fx",
		t: rw(func(s State) {
			r, _ := s.ReadRune()
			s.WriteRune(r)
			// ReadRune may return RuneError, 0. UnreadRune should handle
			// this correctly.
			s.ReadRune()
			s.UnreadRune()
		}),
		nSpan: len("a\u0300\u2208\U0001030fx"),
	}, {
		desc:    "WriteRune, return value.",
		szDst:   5,
		atEOF:   true,
		in:      "a\u0300\u2208\U0001030fx",
		out:     "FaT\u0300",
		outFull: "FaT\u0300F\u2208T\U0001030fTx",
		err:     transform.ErrShortDst,
		t: rwLast(func(s State) bool {
			r, _ := s.ReadRune()
			return s.WriteRune(r)
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "WriteString, return value.",
		szDst:   6,
		atEOF:   true,
		in:      "a\u0300\u2208\U0001030fx",
		out:     "FaT\u0300",
		outFull: "FaT\u0300F\u2208T\U0001030fTx",
		err:     transform.ErrShortDst,
		t: rwLast(func(s State) bool {
			r, _ := s.ReadRune()
			return s.WriteString(string(r))
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "WriteBytes, return value.",
		szDst:   6,
		atEOF:   true,
		in:      "a\u0300\u2208\U0001030fx",
		out:     "FaT\u0300",
		outFull: "FaT\u0300F\u2208T\U0001030fTx",
		err:     transform.ErrShortDst,
		t: rwLast(func(s State) bool {
			r, _ := s.ReadRune()
			return s.WriteBytes([]byte(string(r)))
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Write, size return value.",
		szDst:   6,
		atEOF:   true,
		in:      "a\u0300\u2208\U0001030fx",
		out:     "FaT\u0300",
		outFull: "FaT\u0300F\u2208T\U0001030fTx",
		err:     transform.ErrShortDst,
		t: rwLast(func(s State) bool {
			r, size := s.ReadRune()
			n, _ := s.Write([]byte(string(r)))
			return n == size
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "Write, error return value.",
		szDst:   6,
		atEOF:   true,
		in:      "a\u0300\u2208\U0001030fx",
		out:     "FaT\u0300",
		outFull: "FaT\u0300F\u2208T\U0001030fTx",
		err:     transform.ErrShortDst,
		t: rwLast(func(s State) bool {
			r, _ := s.ReadRune()
			_, err := s.Write([]byte(string(r)))
			return err != transform.ErrShortDst
		}),
		errSpan: transform.ErrEndOfSpan,
	}, {
		desc:    "SetError",
		szDst:   6,
		atEOF:   true,
		in:      "a\u0300\u2208x",
		out:     "a\u0300",
		outFull: "a\u0300",
		err:     myError,
		t: rw(func(s State) {
			r, _ := s.ReadRune()
			if r == 0x2208 {
				s.SetError(myError)
			}
			// Write should have no effect if SetError is called.
			s.WriteRune(r)
		}),
		errSpan: myError,
		nSpan:   len("a\u0300"),
	}}
	for i, tt := range testCases {
		tt.check(t, i)
	}
}

func TestRewriteAlloc(t *testing.T) {
	src := []byte(input)
	dst := make([]byte, len(src))
	r := NewTransformer(&rwCopy{})

	// There should be no allocations.
	if n := testing.AllocsPerRun(1, func() { r.Transform(dst, src, true) }); n > 0 {
		t.Errorf("got %v; want 0", n)
	}
}

func BenchmarkRewriteAll(t *testing.B) {
	dst := make([]byte, 2*len(input))
	src := []byte(input)

	r := NewTransformer(rwReplaceAll{})

	for i := 0; i < t.N; i++ {
		r.Transform(dst, src, true)
	}
}

func BenchmarkRewriteNone(t *testing.B) {
	dst := make([]byte, 2*len(input))
	src := []byte(input)

	r := NewTransformer(rwCopy{})

	for i := 0; i < t.N; i++ {
		r.Transform(dst, src, true)
	}
}
