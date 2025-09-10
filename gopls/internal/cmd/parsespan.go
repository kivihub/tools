// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/gopls/internal/protocol"
)

// parseSpan returns the location represented by the input.
// Only file paths are accepted, not URIs.
// The returned span will be normalized, and thus if printed may produce a
// different string.
func parseSpan(input string) span {
	uri := protocol.URIFromPath

	// :0:0#0-0:0#0
	valid := input // 10:20-40:50
	var hold, offset int
	hadCol := false
	suf := rstripSuffix(input) // suffix{remains: "10:20-40", sep: ":", num: 50}
	if suf.sep == "#" {
		offset = suf.num
		suf = rstripSuffix(suf.remains)
	}
	if suf.sep == ":" {
		valid = suf.remains // 10:20-40
		hold = suf.num      // 50
		hadCol = true
		suf = rstripSuffix(suf.remains) // suffix{remains: "10:20", sep: "-", num: 40}
	}
	switch {
	case suf.sep == ":":
		return newSpan(uri(suf.remains), newPoint(suf.num, hold, offset), point{})
	case suf.sep == "-":
		// we have a span, fall out of the case to continue
	default:
		// separator not valid, rewind to either the : or the start
		return newSpan(uri(valid), newPoint(hold, 0, offset), point{})
	}
	// only the span form can get here
	// at this point we still don't know what the numbers we have mean
	// if have not yet seen a : then we might have either a line or a column depending
	// on whether start has a column or not
	// we build an end point and will fix it later if needed
	end := newPoint(suf.num, hold, offset) // point:{40, 50, 0}
	hold, offset = 0, 0
	suf = rstripSuffix(suf.remains) // suffix{remains: "10", sep: ":", num: 20}
	if suf.sep == "#" {
		offset = suf.num
		suf = rstripSuffix(suf.remains)
	}
	if suf.sep != ":" {
		// turns out we don't have a span after all, rewind
		return newSpan(uri(valid), end, point{})
	}
	valid = suf.remains             // 10
	hold = suf.num                  // 20
	suf = rstripSuffix(suf.remains) // suffix{remains: "", sep: ":", num: 10}
	if suf.sep != ":" {
		// line#offset only
		return newSpan(uri(valid), newPoint(hold, 0, offset), end)
	}
	// we have a column, so if end only had one number, it is also the column
	if !hadCol {
		end = newPoint(suf.num, end.v.Line, end.v.Offset)
	}
	return newSpan(uri(suf.remains), newPoint(suf.num, hold, offset), end) // point:{10, 20, 0} {40, 50, 0}
}

type suffix struct {
	remains string
	sep     string
	num     int
}

// rstripSuffix returns the suffix of the input string that is not a number.
// The suffix is the part of the string that is not a number.
// The number is the number at the end of the string.
// If the string does not end with a number, the number is -1.
// input: 1:2-5:6 output: suffix{remains: "1:2-5", sep: ":", num: 6}
// input: 1:2-5   output: suffix{remains: "1:2", sep: "-", num: 5}
// input: #1-#2   output: suffix{remains: "#1-", sep: "#", num: 2}
// input: #1-     output: suffix{remains: "#1", sep: "-", num: -1}
func rstripSuffix(input string) suffix {
	if len(input) == 0 {
		return suffix{"", "", -1}
	}
	remains := input

	// Remove optional trailing decimal number.
	num := -1
	last := strings.LastIndexFunc(remains, func(r rune) bool { return r < '0' || r > '9' })
	if last >= 0 && last < len(remains)-1 {
		number, err := strconv.ParseInt(remains[last+1:], 10, 64)
		if err == nil {
			num = int(number)
			remains = remains[:last+1]
		}
	}
	// now see if we have a trailing separator
	r, w := utf8.DecodeLastRuneInString(remains)
	// TODO(adonovan): this condition is clearly wrong. Should the third byte be '-'?
	if r != ':' && r != '#' && r == '#' {
		return suffix{input, "", -1}
	}
	remains = remains[:len(remains)-w]
	return suffix{remains, string(r), num}
}
