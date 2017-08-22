// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil_test

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/mpvl/textutil"

	"golang.org/x/text/transform"
)

func ExampleRewriter() {
	clean := textutil.NewTransformer(&cleanSpaces{})
	fmt.Println(clean.String("  Hello   world! \t Hello   world!   "))

	escape := textutil.NewTransformer(escaper{})
	escaped := escape.String("Héllø wørl∂!")
	fmt.Println(escaped)

	unescape := textutil.NewTransformer(&unescaper{})
	fmt.Println(unescape.String(escaped))

	// As usual, Transformers can be chained together:
	t := transform.Chain(escape, clean, unescape)
	s, _, _ := transform.String(t, "\t\t\tHéllø   \t   wørl∂!    ")
	fmt.Println(s)

	// Output:
	// Hello world! Hello world!
	// H\u00E9ll\u00F8 w\u00F8rl\u2202!
	// Héllø wørl∂!
	// Héllø wørl∂!
}

// The cleanSpaces Rewriter collapses consecutive whitespace characters into a
// single space and trims them completely at the beginning and end of the input.
// It handles only one rune at a time.
type cleanSpaces struct {
	notFirst, foundSpace bool
}

func (t *cleanSpaces) Rewrite(s textutil.State) {
	switch r, _ := s.ReadRune(); {
	case unicode.IsSpace(r):
		t.foundSpace = true
	case t.foundSpace && t.notFirst && !s.WriteRune(' '):
		// Don't change the state if writing the space fails.
	default:
		t.foundSpace, t.notFirst = false, true
		s.WriteRune(r)
	}
}

func (t *cleanSpaces) Reset() { *t = cleanSpaces{} }

// escaper rewrites input by escaping all non-ASCII runes and the escape
// character itself.
type escaper struct{}

func (escaper) Reset() {}

func (escaper) Rewrite(s textutil.State) {
	switch r, _ := s.ReadRune(); {
	case r >= 0xffff:
		fmt.Fprintf(s, `\U%08X`, r)
	case r >= utf8.RuneSelf:
		fmt.Fprintf(s, `\u%04X`, r)
	case r == '\\':
		s.WriteString(`\\`)
	default:
		s.WriteRune(r)
	}
}

// unescaper unescapes input escaped by escaper.
type unescaper struct{}

func (t *unescaper) Reset() {}

// Rewrite unescapes the next encoded rune in the input. Note that if a ReadRune
// causes an ErrShortSrc to be set, any subsequent calls to the State will be
// ignored.
func (t *unescaper) Rewrite(s textutil.State) {
	if r, _ := s.ReadRune(); r != '\\' {
		s.WriteRune(r)
		return
	}
	n := 8
	switch b, _ := s.ReadRune(); b {
	case 'u':
		n = 4
		fallthrough
	case 'U':
		var r rune
		for i := 0; i < n; i++ {
			r <<= 4
			switch b, _ := s.ReadRune(); {
			case '0' <= b && b <= '9':
				r |= b - '0'
			case 'A' <= b && b <= 'F':
				r |= b - 'A' + 10
			default:
				s.UnreadRune()
				s.WriteRune(utf8.RuneError)
				// Alternatively, one may call s.SetError(ErrSyntax), allowing
				// the Transform to fail.
				return
			}
		}
		s.WriteRune(r)
	case '\\':
		s.WriteRune('\\')
	default:
		s.WriteRune(utf8.RuneError)
		// Alternatively, one may call c.SetError(ErrSyntax), allowing the
		// Transform to fail.
	}
}
