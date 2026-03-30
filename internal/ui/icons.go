package ui

import (
	"fmt"
	"os"

	"sendtg/internal/domain/entity"
)

var stripEmoji = os.Getenv("SENDTG_NO_EMOJI") == "1"

func titleText() string {
	if stripEmoji {
		return "SendTG - Send File to Telegram"
	}
	return "📤 SendTG - Send File to Telegram"
}

func searchText(query string) string {
	if stripEmoji {
		return fmt.Sprintf("Search: %s", query)
	}
	return fmt.Sprintf("🔍 Search: %s", query)
}

func chatTypeIcon(chatType entity.ChatType) string {
	if stripEmoji {
		switch chatType {
		case entity.ChatTypeGroup, entity.ChatTypeSupergroup:
			return "[G]"
		case entity.ChatTypeChannel:
			return "[C]"
		default:
			return "[U]"
		}
	}

	switch chatType {
	case entity.ChatTypeGroup, entity.ChatTypeSupergroup:
		return "👥"
	case entity.ChatTypeChannel:
		return "📢"
	default:
		return "👤"
	}
}

func pinText() string {
	if stripEmoji {
		return "[PIN] "
	}
	return "📌 "
}
