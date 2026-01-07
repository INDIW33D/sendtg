package entity

import "fmt"

// ChatType represents the type of chat
type ChatType int

const (
	ChatTypePrivate ChatType = iota
	ChatTypeGroup
	ChatTypeSupergroup
	ChatTypeChannel
)

// Chat represents a Telegram chat
type Chat struct {
	ID              int64
	Title           string
	Type            ChatType
	Username        string
	LastMessageDate int  // Unix timestamp of last message
	IsContact       bool // Whether the user is in contacts
	IsBot           bool // Whether the user is a bot
	IsPinned        bool // Whether the chat is pinned
	PinOrder        int  // Order among pinned chats (lower = higher in list)
	Order           int  // Original order from Telegram API (for sorting)
	CanWrite        bool // Whether user can send messages to this chat
}

// UniqueKey returns a unique identifier for the chat
// combining type and ID to handle potential ID collisions across peer types
func (c Chat) UniqueKey() string {
	return fmt.Sprintf("%d:%d", c.Type, c.ID)
}

// DisplayName returns the chat title for display
func (c Chat) DisplayName() string {
	if c.Title != "" {
		return c.Title
	}
	if c.Username != "" {
		return "@" + c.Username
	}
	return "Unknown"
}
