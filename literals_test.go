package goparsify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringLit(t *testing.T) {
	parser := StringLit(`"'`)
	t.Run("test double match", func(t *testing.T) {
		result, p := runParser(`"hello"`, parser)
		require.Equal(t, `hello`, result.Token)
		require.Equal(t, "", p.Get())
	})

	t.Run("test single match", func(t *testing.T) {
		result, p := runParser(`"hello"`, parser)
		require.Equal(t, `hello`, result.Token)
		require.Equal(t, "", p.Get())
	})

	t.Run("test nested quotes", func(t *testing.T) {
		result, p := runParser(`"hello 'world'"`, parser)
		require.Equal(t, `hello 'world'`, result.Token)
		require.Equal(t, "", p.Get())
	})

	t.Run("test non match", func(t *testing.T) {
		_, p := runParser(`1`, parser)
		require.Equal(t, `"'`, p.Error.expected)
		require.Equal(t, `1`, p.Get())
	})

	t.Run("test unterminated string", func(t *testing.T) {
		_, p := runParser(`"hello `, parser)
		require.Equal(t, `"`, p.Error.expected)
		require.Equal(t, `"hello `, p.Get())
	})

	t.Run("test unmatched quotes", func(t *testing.T) {
		_, p := runParser(`"hello '`, parser)
		require.Equal(t, `"`, p.Error.expected)
		require.Equal(t, 0, p.Pos)
	})

	t.Run("test unterminated escape", func(t *testing.T) {
		_, p := runParser(`"hello \`, parser)
		require.Equal(t, `"`, p.Error.expected)
		require.Equal(t, 0, p.Pos)
	})

	t.Run("test escaping", func(t *testing.T) {
		result, p := runParser(`"hello \"w\orld\""`, parser)
		require.Equal(t, `hello "w\orld"`, result.Token)
		require.Equal(t, ``, p.Get())
	})

	t.Run("test escaped whitespace", func(t *testing.T) {
		result, p := runParser(`"hello\tworld"`, parser)
		require.Equal(t, `hello	world`, result.Token)
		require.Equal(t, ``, p.Get())
	})

	t.Run("test unicode chars", func(t *testing.T) {
		result, p := runParser(`"hello 👺 my little goblin"`, parser)
		require.Equal(t, `hello 👺 my little goblin`, result.Token)
		require.Equal(t, ``, p.Get())
	})

	t.Run("test escaped unicode", func(t *testing.T) {
		result, p := runParser(`"hello \ubeef cake"`, parser)
		require.Equal(t, "", p.Error.expected)
		require.Equal(t, "hello \uBEEF cake", result.Token)
		require.Equal(t, ``, p.Get())
	})

	t.Run("test invalid escaped unicode", func(t *testing.T) {
		_, p := runParser(`"hello \ucake"`, parser)
		require.Equal(t, "offset 9: expected [a-f0-9]", p.Error.Error())
		require.Equal(t, 0, p.Pos)
	})

	t.Run("test incomplete escaped unicode", func(t *testing.T) {
		_, p := runParser(`"hello \uca"`, parser)
		require.Equal(t, "offset 9: expected [a-f0-9]{4}", p.Error.Error())
		require.Equal(t, 0, p.Pos)
	})
}

func TestUnicodeStringLiteral(t *testing.T) {
	// TODO(db48x): I really ought to have a few more tests here
	parser := UnicodeStringLiteral()
	t.Run("test “” match", func(t *testing.T) {
		result, p := runParser(`“hello”`, parser)
		require.Equal(t, `hello`, result.Token)
		require.Equal(t, "", p.Get())
	})
	t.Run("test ｢｣ match", func(t *testing.T) {
		result, p := runParser(`｢“hello”｣`, parser)
		require.Equal(t, `“hello”`, result.Token)
		require.Equal(t, "", p.Get())
	})
}

func TestUnhex(t *testing.T) {
	tests := map[int64]string{
		0xF:        "F",
		0x5:        "5",
		0xFF:       "FF",
		0xFFF:      "FFF",
		0xA4B:      "a4b",
		0xFFFF:     "FFFF",
		0xBEEFCAFE: "beeFCAfe",
	}
	for expected, input := range tests {
		t.Run(input, func(t *testing.T) {
			r, ok := unhex(input)
			require.True(t, ok)
			require.EqualValues(t, expected, r)
		})
	}

	t.Run("Fails on non hex chars", func(t *testing.T) {
		_, ok := unhex("hello")
		require.False(t, ok)
	})
}

func TestNumberLit(t *testing.T) {
	parser := NumberLit()
	t.Run("test int", func(t *testing.T) {
		result, p := runParser("1234", parser)
		require.Equal(t, int64(1234), result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("test float", func(t *testing.T) {
		result, p := runParser("12.34", parser)
		require.Equal(t, 12.34, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("test negative float", func(t *testing.T) {
		result, p := runParser("-12.34", parser)
		require.Equal(t, -12.34, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("without leading zero", func(t *testing.T) {
		result, p := runParser("-.34", parser)
		require.Equal(t, -.34, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("scientific notation", func(t *testing.T) {
		result, p := runParser("12.34e3", parser)
		require.Equal(t, 12.34e3, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("scientific notation without decimal", func(t *testing.T) {
		result, p := runParser("34e3", parser)
		require.Equal(t, 34e3, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("scientific notation negative power", func(t *testing.T) {
		result, p := runParser("34e-3", parser)
		require.Equal(t, 34e-3, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("negative scientific notation negative power", func(t *testing.T) {
		result, p := runParser("-.34e-3", parser)
		require.Equal(t, -.34e-3, result.Result)
		require.Equal(t, "", p.Get())
	})

	t.Run("partial match", func(t *testing.T) {
		result, p := runParser("-1.34foo", parser)
		require.Equal(t, -1.34, result.Result)
		require.Equal(t, "foo", p.Get())
	})

	t.Run("non matching string", func(t *testing.T) {
		_, p := runParser("foo", parser)
		require.Equal(t, "offset 0: expected number", p.Error.Error())
		require.Equal(t, 0, p.Pos)
	})

	t.Run("invalid number", func(t *testing.T) {
		_, p := runParser("-.", parser)
		require.Equal(t, "offset 0: expected number", p.Error.Error())
		require.Equal(t, 0, p.Pos)
	})
}

//func RunesFromRange(tab *unicode.RangeTable) <-chan rune {
//	res := make(chan rune)
//	go func() {
//		for _, r16 := range tab.R16 {
//			for c := r16.Lo; c <= r16.Hi; c += r16.Stride {
//				res <- rune(c)
//			}
//		}
//		for _, r32 := range tab.R32 {
//			for c := r32.Lo; c <= r32.Hi; c += r32.Stride {
//				res <- rune(c)
//			}
//		}
//		close(res)
//	}()
//	return res
//}
