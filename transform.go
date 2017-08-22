// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import "golang.org/x/text/transform"

// A Transformer wraps a transform.SpanningTransformer providing convenience
// methods for most of the functionality in the tranform package.
type Transformer struct {
	transform.SpanningTransformer
}

// Transform calls the Transform method of the underlying Transformer.
func (t Transformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	return t.SpanningTransformer.Transform(dst, src, atEOF)
}

// Span calls the Span method of the underlying Transformer.
func (t Transformer) Span(b []byte, atEOF bool) (n int, err error) {
	return t.SpanningTransformer.Span(b, atEOF)
}

// Reset calls the Reset method of the underlying Transformer.
func (t Transformer) Reset() { t.SpanningTransformer.Reset() }

// String applies t to s and returns the result. This methods wraps
// transform.String. It returns the empty string if any error occurred.
func (t Transformer) String(s string) string {
	s, _, err := transform.String(t.SpanningTransformer, s)
	if err != nil {
		return ""
	}
	return s
}

// Bytes returns a new byte slice with the result of converting b using t. It
// calls Reset on t. It returns nil if any error was found.
func (t Transformer) Bytes(b []byte) []byte {
	b, _, err := transform.Bytes(t, b)
	if err != nil {
		return nil
	}
	return b
}
