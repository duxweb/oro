package naming

import (
	"strings"
	"unicode"
)

func Snake(name string) string {
	var builder strings.Builder
	runes := []rune(name)
	for index, r := range runes {
		if index > 0 && unicode.IsUpper(r) && shouldSplitWord(runes, index) {
			builder.WriteByte('_')
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

func shouldSplitWord(runes []rune, index int) bool {
	previous := runes[index-1]
	if unicode.IsLower(previous) || unicode.IsDigit(previous) {
		return true
	}
	nextIndex := index + 1
	return nextIndex < len(runes) && unicode.IsLower(runes[nextIndex])
}
