package entity

// Folder represents a Telegram chat folder
type Folder struct {
	ID    int32
	Title string

	// Filter settings
	IncludeContacts    bool
	IncludeNonContacts bool
	IncludeGroups      bool
	IncludeBroadcasts  bool
	IncludeBots        bool

	// Specific peers included/excluded/pinned
	IncludedPeerIDs []int64
	ExcludedPeerIDs []int64
	PinnedPeerIDs   []int64 // Pinned chats for this folder (in order)
}

// DisplayName returns the folder title
func (f Folder) DisplayName() string {
	return f.Title
}

// IsAllChats returns true if this is the "All Chats" folder
func (f Folder) IsAllChats() bool {
	return f.ID == 0
}

// IsPinnedInFolder checks if a chat is pinned in this folder
func (f Folder) IsPinnedInFolder(chatID int64) (bool, int) {
	for i, id := range f.PinnedPeerIDs {
		if id == chatID {
			return true, i
		}
	}
	return false, -1
}

// ContainsChat checks if a chat belongs to this folder
func (f Folder) ContainsChat(chat Chat, isContact bool) bool {
	// "All Chats" folder contains everything
	if f.IsAllChats() {
		return true
	}

	chatID := chat.ID

	// Check if explicitly excluded
	for _, id := range f.ExcludedPeerIDs {
		if id == chatID {
			return false
		}
	}

	// Check if pinned in this folder - pinned chats are always included
	for _, id := range f.PinnedPeerIDs {
		if id == chatID {
			return true
		}
	}

	// Check if explicitly included
	for _, id := range f.IncludedPeerIDs {
		if id == chatID {
			return true
		}
	}

	// Check by type
	switch chat.Type {
	case ChatTypePrivate:
		// Check if it's a bot
		if chat.IsBot {
			return f.IncludeBots
		}
		// Check if it's a contact or non-contact
		if isContact && f.IncludeContacts {
			return true
		}
		if !isContact && f.IncludeNonContacts {
			return true
		}
	case ChatTypeGroup, ChatTypeSupergroup:
		if f.IncludeGroups {
			return true
		}
	case ChatTypeChannel:
		if f.IncludeBroadcasts {
			return true
		}
	}

	return false
}
