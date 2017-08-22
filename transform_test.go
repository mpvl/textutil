// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"strings"
	"testing"

	"golang.org/x/text/transform"
)

// copied from golang.org/x/text/runes

type transformTest struct {
	desc    string
	szDst   int
	atEOF   bool
	repl    string
	in      string
	out     string // result string of first call to Transform
	outFull string // transform of entire input string
	err     error
	errSpan error
	nSpan   int

	// t transform.SpanningTransformer
	t transform.SpanningTransformer
}

const large = 10240

var input = strings.Repeat("Thé qüick brøwn føx jumps øver the lazy døg. ", 100)

func (tt *transformTest) check(t *testing.T, i int) {
	if tt.t == nil {
		return
	}
	dst := make([]byte, tt.szDst)
	src := []byte(tt.in)
	nDst, nSrc, err := tt.t.Transform(dst, src, tt.atEOF)
	if err != tt.err {
		t.Errorf("%d:%s:error: got %v; want %v", i, tt.desc, err, tt.err)
	}
	if got := string(dst[:nDst]); got != tt.out {
		t.Errorf("%d:%s:out: got %q; want %q", i, tt.desc, got, tt.out)
	}

	// Calls tt.t.Transform for the remainder of the input. We use this to test
	// the nSrc return value.
	out := make([]byte, large)
	n := copy(out, dst[:nDst])
	nDst, _, _ = tt.t.Transform(out[n:], src[nSrc:], true)
	if got, want := string(out[:n+nDst]), tt.outFull; got != want {
		t.Errorf("%d:%s:outFull: got %q; want %q", i, tt.desc, got, want)
	}

	tt.t.Reset()
	p := 0
	for ; p < len(tt.in) && p < len(tt.outFull) && tt.in[p] == tt.outFull[p]; p++ {
	}
	if tt.nSpan != 0 {
		p = tt.nSpan
	}
	if n, err = tt.t.Span([]byte(tt.in), tt.atEOF); n != p || err != tt.errSpan {
		t.Errorf("%d:%s:span: got %d, %v; want %d, %v", i, tt.desc, n, err, p, tt.errSpan)
	}
}
