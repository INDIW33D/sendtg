package entity

import "fmt"

// FolderKind identifies the logical folder type shown in the UI.
type FolderKind string

const (
	FolderKindAll     FolderKind = "all"
	FolderKindCustom  FolderKind = "custom"
	FolderKindArchive FolderKind = "archive"
)

// Folder represents a Telegram chat folder
type Folder struct {
	Kind  FolderKind
	ID    int32
	Title string

	// Filter settings
	IncludeContacts    bool
	IncludeNonContacts bool
	IncludeGroups      bool
	IncludeBroadcasts  bool
	IncludeBots        bool
	ExcludeMuted       bool
	ExcludeRead        bool
	ExcludeArchived    bool

	// Specific peers included/excluded/pinned
	IncludedPeers []PeerRef
	ExcludedPeers []PeerRef
	PinnedPeers   []PeerRef // Pinned chats for this folder (in order)
}

// DisplayName returns the folder title
func (f Folder) DisplayName() string {
	if f.IsArchive() && f.Title == "" {
		return "Archive"
	}
	return f.Title
}

// IsAllChats returns true if this is the "All Chats" folder
func (f Folder) IsAllChats() bool {
	return f.Kind == FolderKindAll
}

// IsArchive returns true if this is the archive peer folder.
func (f Folder) IsArchive() bool {
	return f.Kind == FolderKindArchive
}

// Key returns a stable identifier for selection persistence.
func (f Folder) Key() string {
	switch f.Kind {
	case FolderKindAll:
		return string(FolderKindAll)
	case FolderKindArchive:
		return string(FolderKindArchive)
	default:
		return fmt.Sprintf("%s:%d", FolderKindCustom, f.ID)
	}
}

// IsPinnedInFolder checks if a chat is pinned in this folder
func (f Folder) IsPinnedInFolder(peer PeerRef) (bool, int) {
	for i, pinnedPeer := range f.PinnedPeers {
		if pinnedPeer.Matches(peer) {
			return true, i
		}
	}
	return false, -1
}

// ContainsChat checks if a chat belongs to this folder
func (f Folder) ContainsChat(chat Chat) bool {
	// "All Chats" folder contains everything
	if f.IsAllChats() {
		return !chat.IsArchived
	}

	if f.IsArchive() {
		return chat.IsArchived
	}

	// Check if explicitly excluded
	for _, peer := range f.ExcludedPeers {
		if peer.Matches(chat.Peer) {
			return false
		}
	}

	// Check if pinned in this folder - pinned chats are always included
	for _, peer := range f.PinnedPeers {
		if peer.Matches(chat.Peer) {
			return true
		}
	}

	// Check if explicitly included
	for _, peer := range f.IncludedPeers {
		if peer.Matches(chat.Peer) {
			return true
		}
	}

	if f.ExcludeArchived && chat.IsArchived {
		return false
	}

	if f.ExcludeMuted && chat.IsMuted {
		return false
	}

	if f.ExcludeRead && !chat.HasUnread() {
		return false
	}

	// Check by type
	switch chat.Type {
	case ChatTypePrivate:
		// Check if it's a bot
		if chat.IsBot {
			return f.IncludeBots
		}
		// Check if it's a contact or non-contact
		if chat.IsContact && f.IncludeContacts {
			return true
		}
		if !chat.IsContact && f.IncludeNonContacts {
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
