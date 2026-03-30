package telegram

import (
	"testing"

	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
)

func TestInputPeerFromDialogPeerUsesInputPeerSelf(t *testing.T) {
	peer := inputPeerFromDialogPeer(&tg.PeerUser{UserID: 42}, map[int64]*tg.User{
		42: {ID: 42, Self: true},
	}, nil)

	if _, ok := peer.(*tg.InputPeerSelf); !ok {
		t.Fatalf("expected InputPeerSelf, got %#v", peer)
	}
}

func TestExtractPeersResolvesInputPeerSelf(t *testing.T) {
	refs := extractPeers([]tg.InputPeerClass{&tg.InputPeerSelf{}}, 42)
	if len(refs) != 1 {
		t.Fatalf("expected one peer ref, got %d", len(refs))
	}
	if refs[0] != (entity.PeerRef{Kind: entity.PeerKindUser, ID: 42}) {
		t.Fatalf("unexpected peer ref: %+v", refs[0])
	}
}
