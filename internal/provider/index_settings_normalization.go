package provider

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Charabia's non-lossy string pipeline applies Persian mappings globally.
func indexSettingsStopWordKey(value string) string {
	var normalized strings.Builder

	for _, character := range norm.NFKD.String(value) {
		if unicode.IsControl(character) && !unicode.IsSpace(character) {
			continue
		}

		if character >= '۰' && character <= '۹' {
			normalized.WriteRune('0' + character - '۰')
			continue
		}

		switch character {
		case '\u200c':
			continue
		case 'ي', 'ی', 'ى', 'ۀ':
			character = 'ی'
		case 'ك', 'ک':
			character = 'ک'
		case '،':
			character = ','
		case '؟':
			character = '?'
		case '\ufdfc':
			normalized.WriteString("RIAL")
			continue
		}

		normalized.WriteRune(character)
	}

	return normalized.String()
}
