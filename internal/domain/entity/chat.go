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
	Peer            PeerRef
	Title           string
	Type            ChatType
	Username        string
	LastMessageDate int  // Unix timestamp of last message
	IsContact       bool // Whether the user is in contacts
	IsBot           bool // Whether the user is a bot
	IsArchived      bool // Whether the chat belongs to the archive peer folder
	IsMuted         bool // Whether notifications are muted for the chat
	UnreadCount     int  // Number of unread messages
	UnreadMarked    bool // Whether the chat was manually marked as unread
	IsPinned        bool // Whether the chat is pinned
	PinOrder        int  // Order among pinned chats (lower = higher in list)
	Order           int  // Original order from Telegram API (for sorting)
	CanWrite        bool // Whether user can send messages to this chat
}

// HasUnread reports whether the chat should be treated as unread by folder rules.
func (c Chat) HasUnread() bool {
	return c.UnreadCount > 0 || c.UnreadMarked
}

// UniqueKey returns a unique identifier for the chat
// combining type and ID to handle potential ID collisions across peer types
func (c Chat) UniqueKey() string {
	if key := c.Peer.Key(); key != "" {
		return key
	}
	return fmt.Sprintf("%d:%s", c.Type, c.Title)
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
