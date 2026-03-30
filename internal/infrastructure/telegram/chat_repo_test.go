package telegram

import (
	"testing"
	"time"

	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
)

func TestProcessDialogsKeepsDistinctTypedPeers(t *testing.T) {
	repo := &ChatRepository{}

	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}},
		&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 1}},
	}
	messages := []tg.MessageClass{
		&tg.Message{ID: 10, PeerID: &tg.PeerUser{UserID: 1}, Date: 111},
		&tg.Message{ID: 20, PeerID: &tg.PeerChannel{ChannelID: 1}, Date: 222},
	}
	users := []tg.UserClass{
		&tg.User{ID: 1, AccessHash: 1111, FirstName: "Alice"},
	}
	chats := []tg.ChatClass{
		&tg.Channel{ID: 1, AccessHash: 2222, Title: "News"},
	}

	processed, err := repo.processDialogs(dialogs, messages, chats, users, map[int64]bool{}, mainPeerFolderID)
	if err != nil {
		t.Fatalf("processDialogs returned error: %v", err)
	}
	if len(processed) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(processed))
	}

	got := map[string]entity.Chat{}
	for _, chat := range processed {
		got[chat.UniqueKey()] = chat
	}

	userChat, ok := got[entity.PeerRef{Kind: entity.PeerKindUser, ID: 1}.Key()]
	if !ok {
		t.Fatal("expected private chat keyed by user peer")
	}
	if userChat.LastMessageDate != 111 {
		t.Fatalf("expected user chat message date 111, got %d", userChat.LastMessageDate)
	}

	channelChat, ok := got[entity.PeerRef{Kind: entity.PeerKindChannel, ID: 1}.Key()]
	if !ok {
		t.Fatal("expected channel chat keyed by channel peer")
	}
	if channelChat.LastMessageDate != 222 {
		t.Fatalf("expected channel chat message date 222, got %d", channelChat.LastMessageDate)
	}
	if channelChat.Peer.AccessHash != 2222 {
		t.Fatalf("expected channel access hash to be preserved, got %d", channelChat.Peer.AccessHash)
	}
}

func TestProcessDialogsCapturesArchiveMuteAndUnreadState(t *testing.T) {
	repo := &ChatRepository{}

	futureMute := int(time.Now().Add(time.Hour).Unix())
	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer:           &tg.PeerUser{UserID: 1},
			TopMessage:     10,
			UnreadCount:    2,
			UnreadMark:     true,
			NotifySettings: tg.PeerNotifySettings{},
			FolderID:       int(archivePeerFolderID),
		},
	}
	dialog := dialogs[0].(*tg.Dialog)
	dialog.NotifySettings.SetMuteUntil(futureMute)
	dialog.SetFolderID(int(archivePeerFolderID))

	messages := []tg.MessageClass{
		&tg.Message{ID: 10, PeerID: &tg.PeerUser{UserID: 1}, Date: 111},
	}
	users := []tg.UserClass{
		&tg.User{ID: 1, AccessHash: 1111, FirstName: "Alice"},
	}

	processed, err := repo.processDialogs(dialogs, messages, nil, users, map[int64]bool{}, archivePeerFolderID)
	if err != nil {
		t.Fatalf("processDialogs returned error: %v", err)
	}
	if len(processed) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(processed))
	}

	chat := processed[0]
	if !chat.IsArchived {
		t.Fatal("expected chat to be marked archived")
	}
	if !chat.IsMuted {
		t.Fatal("expected chat to be marked muted")
	}
	if chat.UnreadCount != 2 || !chat.UnreadMarked {
		t.Fatalf("expected unread state to be preserved, got count=%d marked=%v", chat.UnreadCount, chat.UnreadMarked)
	}
}

func TestMergeChatInventoriesKeepsArchiveAfterMain(t *testing.T) {
	mainChats := []entity.Chat{{Peer: entity.PeerRef{Kind: entity.PeerKindUser, ID: 1}, Order: 5}}
	archiveChats := []entity.Chat{{Peer: entity.PeerRef{Kind: entity.PeerKindUser, ID: 2}, IsArchived: true, Order: 0}}

	merged := mergeChatInventories(mainChats, archiveChats)
	if len(merged) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(merged))
	}
	if merged[0].Peer.ID != 1 || merged[0].Order != 0 {
		t.Fatalf("expected first chat to be main inventory chat with normalized order, got %+v", merged[0])
	}
	if merged[1].Peer.ID != 2 || merged[1].Order != 1 || !merged[1].IsArchived {
		t.Fatalf("expected second chat to be archived inventory chat, got %+v", merged[1])
	}
}

func TestNextDialogsPageOffsetSkipsNonPaginatableTailDialogs(t *testing.T) {
	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}, TopMessage: 10},
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 2}, TopMessage: 0},
	}
	messages := []tg.MessageClass{
		&tg.Message{ID: 10, PeerID: &tg.PeerUser{UserID: 1}, Date: 123},
	}
	users := buildUserMap([]tg.UserClass{
		&tg.User{ID: 1, AccessHash: 1111, FirstName: "Alice"},
		&tg.User{ID: 2, AccessHash: 2222, FirstName: "Bob"},
	})

	nextDate, nextID, nextPeer, ok := nextDialogsPageOffset(dialogs, messages, users, nil)
	if !ok {
		t.Fatal("expected pagination offset to be found")
	}
	if nextDate != 123 || nextID != 10 {
		t.Fatalf("expected fallback to first paginatable dialog, got date=%d id=%d", nextDate, nextID)
	}
	peer, ok := nextPeer.(*tg.InputPeerUser)
	if !ok || peer.UserID != 1 {
		t.Fatalf("expected input peer for first dialog, got %#v", nextPeer)
	}
}
