package entity

import "testing"

func TestFolderTypedPeerMatching(t *testing.T) {
	folder := Folder{
		Kind:          FolderKindCustom,
		ID:            7,
		IncludedPeers: []PeerRef{{Kind: PeerKindUser, ID: 42}},
		ExcludedPeers: []PeerRef{{Kind: PeerKindChannel, ID: 42}},
		PinnedPeers:   []PeerRef{{Kind: PeerKindChannel, ID: 99}},
	}

	userChat := Chat{Peer: PeerRef{Kind: PeerKindUser, ID: 42}, Type: ChatTypePrivate}
	channelChat := Chat{Peer: PeerRef{Kind: PeerKindChannel, ID: 42}, Type: ChatTypeChannel}
	otherChannel := Chat{Peer: PeerRef{Kind: PeerKindChannel, ID: 99}, Type: ChatTypeChannel}

	if !folder.ContainsChat(userChat) {
		t.Fatal("expected included user chat to match folder")
	}
	if folder.ContainsChat(channelChat) {
		t.Fatal("expected excluded channel chat with same numeric ID to be filtered out")
	}
	if pinned, order := folder.IsPinnedInFolder(otherChannel.Peer); !pinned || order != 0 {
		t.Fatalf("expected pinned channel peer to be detected, got pinned=%v order=%d", pinned, order)
	}
	if pinned, _ := folder.IsPinnedInFolder(userChat.Peer); pinned {
		t.Fatal("expected different peer kind with same ID to not be pinned")
	}
}

func TestFolderContainsChatRespectsArchiveMutedAndUnreadFlags(t *testing.T) {
	folder := Folder{
		Kind:            FolderKindCustom,
		ID:              8,
		IncludeGroups:   true,
		ExcludeArchived: true,
		ExcludeMuted:    true,
		ExcludeRead:     true,
	}

	visible := Chat{Type: ChatTypeGroup, UnreadCount: 3}
	archived := Chat{Type: ChatTypeGroup, IsArchived: true, UnreadCount: 3}
	muted := Chat{Type: ChatTypeGroup, IsMuted: true, UnreadCount: 3}
	read := Chat{Type: ChatTypeGroup}
	markedUnread := Chat{Type: ChatTypeGroup, UnreadMarked: true}

	if !folder.ContainsChat(visible) {
		t.Fatal("expected visible unread unmuted chat to match folder")
	}
	if folder.ContainsChat(archived) {
		t.Fatal("expected archived chat to be excluded")
	}
	if folder.ContainsChat(muted) {
		t.Fatal("expected muted chat to be excluded")
	}
	if folder.ContainsChat(read) {
		t.Fatal("expected fully read chat to be excluded")
	}
	if !folder.ContainsChat(markedUnread) {
		t.Fatal("expected manually unread-marked chat to match folder")
	}
}

func TestBuiltInFoldersHandleArchiveState(t *testing.T) {
	allChats := Folder{Kind: FolderKindAll, Title: "All Chats"}
	archive := Folder{Kind: FolderKindArchive, Title: "Archive"}

	mainChat := Chat{Type: ChatTypePrivate}
	archivedChat := Chat{Type: ChatTypePrivate, IsArchived: true}

	if !allChats.ContainsChat(mainChat) {
		t.Fatal("expected main chat in all chats")
	}
	if allChats.ContainsChat(archivedChat) {
		t.Fatal("expected archived chat to stay out of all chats")
	}
	if archive.ContainsChat(mainChat) {
		t.Fatal("expected main chat to stay out of archive")
	}
	if !archive.ContainsChat(archivedChat) {
		t.Fatal("expected archived chat in archive folder")
	}
}
