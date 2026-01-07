package telegram

import (
	"os"
	"strings"

	"github.com/gotd/td/tg"
)

// Strip emoji flag - set to true to remove all emoji (for terminals without emoji support)
var stripEmoji = os.Getenv("SENDTG_NO_EMOJI") == "1"

// ExtractTextWithCustomEmoji extracts text from TextWithEntities
// The text already contains fallback emoji from Telegram, we just clean invisible chars
func ExtractTextWithCustomEmoji(t tg.TextWithEntities) string {
	return cleanInvisibleChars(t.Text)
}

// cleanInvisibleChars removes invisible placeholder characters used by Telegram
// If SENDTG_NO_EMOJI=1, also removes all emoji for terminals without emoji support
func cleanInvisibleChars(s string) string {
	runes := []rune(s)
	result := make([]rune, 0, len(runes))

	for _, r := range runes {
		switch {
		case r == '\uFFFC' || r == '\uFFFD':
			continue
		case r == '\u200B' || r == '\u200C' || r == '\u2060' || r == '\uFEFF':
			continue
		case r == '\u2800':
			continue
		case r >= 0xE000 && r <= 0xF8FF:
			// Private Use Area - skip
			continue
		case r >= 0xFE00 && r <= 0xFE0F:
			// Variation Selectors - keep only FE0F for emoji
			if stripEmoji || r != 0xFE0F {
				continue
			}
			result = append(result, r)
		case stripEmoji && isEmojiRune(r):
			continue
		default:
			result = append(result, r)
		}
	}

	return strings.TrimSpace(string(result))
}

// isEmojiRune checks if a rune is likely an emoji
func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F600 && r <= 0x1F64F: // Emoticons
		return true
	case r >= 0x1F300 && r <= 0x1F5FF: // Misc Symbols and Pictographs
		return true
	case r >= 0x1F680 && r <= 0x1F6FF: // Transport and Map
		return true
	case r >= 0x1F900 && r <= 0x1F9FF: // Supplemental Symbols and Pictographs
		return true
	case r >= 0x1FA00 && r <= 0x1FAFF: // Symbols and Pictographs Extended-A
		return true
	case r >= 0x2600 && r <= 0x26FF: // Misc symbols
		return true
	case r >= 0x2700 && r <= 0x27BF: // Dingbats
		return true
	case r >= 0x2300 && r <= 0x23FF: // Misc Technical
		return true
	case r >= 0x2B50 && r <= 0x2B55: // Stars, circles
		return true
	}
	return false
}
