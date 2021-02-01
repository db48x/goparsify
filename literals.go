package goparsify

import (
	"bytes"
	"strconv"
	"unicode"
	"unicode/utf8"
)

func stringImpl(ps *State, node *Result, closer rune, escapes map[rune]rune) bool {
	var end = node.Start

	inputLen := len(ps.Input)
	var buf *bytes.Buffer

	for end < inputLen {
		current, size := utf8.DecodeRuneInString(ps.Input[end:])
		switch current {
		case '\\':
			if end+size >= inputLen {
				ps.ErrorHere(string(closer))
				return false
			}

			if buf == nil {
				buf = bytes.NewBufferString(ps.Input[node.Start:end])
			}

			c, s := utf8.DecodeRuneInString(ps.Input[end+size:])
			if c == 'u' {
				if end+size+s+4 >= inputLen {
					ps.Error.expected = "[a-f0-9]{4}"
					ps.Error.pos = end + size + s
					return false
				}

				r, ok := unhex(ps.Input[end+size+s : end+size+s+4])
				if !ok {
					ps.Error.expected = "[a-f0-9]"
					ps.Error.pos = end + size + s
					return false
				}
				buf.WriteRune(r)
				end += size + s + 4
			} else {
				if c == closer {
					buf.WriteRune(c)
				} else {
					replacement, ok := escapes[c]
					if ok {
						buf.WriteRune(replacement)
					} else {
						// write both the slash and the following character
						buf.WriteRune(current)
						buf.WriteRune(c)
					}
				}
				end += size + s
			}
		case closer:
			if buf == nil {
				node.Token = ps.Input[node.Start:end]
				ps.Pos = end + size
				return true
			}
			ps.Pos = end + size
			node.Token = buf.String()
			return true
		default:
			end += size
			if buf != nil {
				buf.WriteRune(current)
			}
		}
	}
	ps.ErrorHere(string(closer))
	return false
}

// StringLit matches a quoted string and returns it in .Token. It may contain:
//  - unicode
//  - escaped characters, eg \", \n, \t
//  - unicode sequences, eg \uBEEF
// allowedQuotes is the list of allowed quote characters; both the
// opening and closing quotes will be the same character from this
// string
func StringLit(allowedQuotes string) Parser {
	return NewParser("string literal", func(ps *State, node *Result) {
		ps.WS(ps)

		opener, size := utf8.DecodeRuneInString(ps.Input[ps.Pos:])
		if !stringContainsRune(allowedQuotes, opener) {
			ps.ErrorHere(allowedQuotes)
			return
		}
		node.Start = ps.Pos + size
		stringImpl(ps, node, opener, _Escapes)
	})
}

// UnicodeStringLiteral matches a quoted string and returns it in .Token. It may contain:
//  - unicode
//  - escaped characters, eg \", \n, \t
//  - unicode sequences, eg \uBEEF
// The opening and closing quote character may be any matched pair of
// unicode characters from the Pi/Pf categories, or from the Ps/Pe
// categories, plus angle brackets, or if they may be a punctuation
// character, as long as they are the same punctuation character.
func UnicodeStringLiteral() Parser {
	return CustomStringLiteral(IsValidRegexpDelimiter, _Escapes)
}

// CustomStringLiteral matches a quoted string and returns it in .Token. It may contain:
//  - unicode
//  - escaped characters, eg \", \n, \t
//  - unicode sequences, eg \uBEEF
// The opening and closing quotes are validated by the isValid
// function you pass in. This function should return true if its
// argument is a valid opening quote character, plus the correct
// closing quote character that ends the string. See
// IsValidRegexpDelimiter.
//
// The only valid escape characters are those defined in the escapes
// argument, plus one for the closer returned by isValid.
func CustomStringLiteral(isValid func(rune) (bool, rune), escapes map[rune]rune) Parser {
	return NewParser("string literal", func(ps *State, node *Result) {
		ps.WS(ps)

		opener, size := utf8.DecodeRuneInString(ps.Input[ps.Pos:])
		valid, closer := isValid(opener)
		if !valid {
			ps.ErrorHere("string delimiter")
			return
		}
		node.Start = ps.Pos + size
		matched := stringImpl(ps, node, closer, escapes)
		if !matched {
			ps.ErrorHere(string("string delimiter"))
		}
	})
}

func UnicodeRegexpMatchLiteral() Parser {
	return CustomRegexpMatchLiteral(IsValidRegexpDelimiter, _Escapes)
}

func CustomRegexpMatchLiteral(isValid func(rune) (bool, rune), escapes map[rune]rune) Parser {
	return NewParser("regexp match literal", func(ps *State, node *Result) {
		ps.WS(ps)

		opener, size := utf8.DecodeRuneInString(ps.Input[ps.Pos:])
		valid, closer := isValid(opener)
		if !valid {
			ps.ErrorHere("regexp delimiter")
			return
		}
		node.Start = ps.Pos + size
		matched := stringImpl(ps, node, closer, escapes)
		if !matched {
			ps.ErrorHere(string(closer))
		}
	})
}

func UnicodeRegexpReplaceLiteral() Parser {
	return CustomRegexpReplaceLiteral(IsValidRegexpDelimiter, _Escapes)
}

func CustomRegexpReplaceLiteral(isValid func(rune) (bool, rune), escapes map[rune]rune) Parser {
	return NewParser("regexp replace literal", func(ps *State, node *Result) {
		ps.WS(ps)

		child1 := *node
		opener, size := utf8.DecodeRuneInString(ps.Input[ps.Pos:])
		valid, closer := isValid(opener)
		if !valid {
			ps.ErrorHere("regexp delimiter")
			return
		}
		ps.Pos += size
		child1.Start = ps.Pos

		matched := stringImpl(ps, &child1, closer, escapes)
		if !matched {
			ps.ErrorHere(string(closer))
			return
		}

		child2 := *node
		if closer != opener {
			opener, size = utf8.DecodeRuneInString(ps.Input[ps.Pos:])
			valid, closer = IsValidRegexpDelimiter(opener)
			ps.Pos += size
			if !valid {
				ps.ErrorHere("regexp delimiter")
				return
			}
		}
		child2.Start = ps.Pos

		matched = stringImpl(ps, &child2, closer, _Escapes)
		if !matched {
			ps.ErrorHere(string(closer))
			return
		}

		node.Child = []Result{child1, child2}
	})
}

// NumberLit matches a floating point or integer number and returns it as a int64 or float64 in .Result
func NumberLit() Parser {
	return NewParser("number literal", func(ps *State, node *Result) {
		ps.WS(ps)
		end := ps.Pos
		float := false
		inputLen := len(ps.Input)

		if end < inputLen && (ps.Input[end] == '-' || ps.Input[end] == '+') {
			end++
		}

		for end < inputLen && ps.Input[end] >= '0' && ps.Input[end] <= '9' {
			end++
		}

		if end < inputLen && ps.Input[end] == '.' {
			float = true
			end++
		}

		for end < inputLen && ps.Input[end] >= '0' && ps.Input[end] <= '9' {
			end++
		}

		if end < inputLen && (ps.Input[end] == 'e' || ps.Input[end] == 'E') {
			end++
			float = true

			if end < inputLen && (ps.Input[end] == '-' || ps.Input[end] == '+') {
				end++
			}

			for end < inputLen && ps.Input[end] >= '0' && ps.Input[end] <= '9' {
				end++
			}
		}

		if end == ps.Pos {
			ps.ErrorHere("number")
			return
		}

		var err error
		if float {
			node.Result, err = strconv.ParseFloat(ps.Input[ps.Pos:end], 10)
		} else {
			node.Result, err = strconv.ParseInt(ps.Input[ps.Pos:end], 10, 64)
		}
		if err != nil {
			ps.ErrorHere("number")
			return
		}
		node.Start = ps.Pos
		node.End = end
		ps.Pos = end
	})
}

func stringContainsRune(s string, r rune) bool {
	runes := bytes.Runes([]byte(s))
	for _, candidate := range runes {
		if candidate == r {
			return true
		}
	}
	return false
}

func unhex(b string) (v rune, ok bool) {
	for _, c := range b {
		v <<= 4
		switch {
		case '0' <= c && c <= '9':
			v |= c - '0'
		case 'a' <= c && c <= 'f':
			v |= c - 'a' + 10
		case 'A' <= c && c <= 'F':
			v |= c - 'A' + 10
		default:
			return 0, false
		}
	}

	return v, true
}

var _PiPf = map[rune]rune{
	'«': '»', '‘': '’', '“': '”', '‹': '›', '⸂': '⸃', '⸄': '⸅', '⸉': '⸊',
	'⸌': '⸍', '⸜': '⸝', '⸠': '⸡',
}

var _PsPf = map[rune]rune{
	'‚': '’', '„': '”',
}

var _PsPe = map[rune]rune{
	'(': ')', '[': ']', '{': '}', '༺': '༻', '༼': '༽', '᚛': '᚜', '⁅': '⁆',
	'⁽': '⁾', '₍': '₎', '❨': '❩', '❪': '❫', '❬': '❭', '❮': '❯', '❰': '❱',
	'❲': '❳', '❴': '❵', '⟅': '⟆', '⟦': '⟧', '⟨': '⟩', '⟪': '⟫', '⦃': '⦄',
	'⦅': '⦆', '⦇': '⦈', '⦉': '⦊', '⦋': '⦌', '⦑': '⦒', '⦓': '⦔', '⦕': '⦖',
	'⦗': '⦘', '⧘': '⧙', '⧚': '⧛', '⧼': '⧽', '〈': '〉', '《': '》',
	'「': '」', '『': '』', '【': '】', '〔': '〕', '〖': '〗', '〘': '〙',
	'〚': '〛', '〝': '〞', '︗': '︘', '︵': '︶', '︷': '︸', '︹': '︺',
	'︻': '︼', '︽': '︾', '︿': '﹀', '﹁': '﹂', '﹃': '﹄', '﹇': '﹈',
	'﹙': '﹚', '﹛': '﹜', '﹝': '﹞', '（': '）', '［': '］', '｛': '｝',
	'｟': '｠', '｢': '｣', '⸨': '⸩',
}

var _SmSm = map[rune]rune{
	'<': '>',
}

var _Escapes = map[rune]rune{
	'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t',
	'v': '\v',
}

// IsValidRegexpDelimiter allows quote taken from the set of unicode
// punctuation characters, plus angle brackets (which are actually
// math symbols). It ensures that the closing quote character will be
// symmetrically paired with the opening character if possible.
func IsValidRegexpDelimiter(r rune) (bool, rune) {
	var close rune
	var exists bool
	for _, m := range []map[rune]rune{_PiPf, _PsPf, _PsPe, _SmSm} {
		close, exists = m[r]
		if exists {
			return true, close
		}
	}
	if unicode.IsPunct(r) || r == '>' {
		return true, r
	}
	return false, -1
}
