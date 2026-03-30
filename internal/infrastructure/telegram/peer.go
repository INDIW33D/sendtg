package telegram

import (
	"fmt"

	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
)

func extractPeers(peers []tg.InputPeerClass, selfID int64) []entity.PeerRef {
	refs := make([]entity.PeerRef, 0, len(peers))
	for _, peer := range peers {
		if ref, ok := peerRefFromInputPeer(peer, selfID); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func peerRefFromInputPeer(peer tg.InputPeerClass, selfID int64) (entity.PeerRef, bool) {
	switch p := peer.(type) {
	case *tg.InputPeerSelf:
		if selfID == 0 {
			return entity.PeerRef{}, false
		}
		return entity.PeerRef{Kind: entity.PeerKindUser, ID: selfID}, true
	case *tg.InputPeerUser:
		return entity.PeerRef{Kind: entity.PeerKindUser, ID: p.UserID, AccessHash: p.AccessHash}, true
	case *tg.InputPeerChat:
		return entity.PeerRef{Kind: entity.PeerKindChat, ID: p.ChatID}, true
	case *tg.InputPeerChannel:
		return entity.PeerRef{Kind: entity.PeerKindChannel, ID: p.ChannelID, AccessHash: p.AccessHash}, true
	case *tg.InputPeerUserFromMessage:
		return entity.PeerRef{Kind: entity.PeerKindUser, ID: p.UserID}, true
	case *tg.InputPeerChannelFromMessage:
		return entity.PeerRef{Kind: entity.PeerKindChannel, ID: p.ChannelID}, true
	default:
		return entity.PeerRef{}, false
	}
}

func inputPeerFromPeerRef(peer entity.PeerRef) (tg.InputPeerClass, error) {
	switch peer.Kind {
	case entity.PeerKindUser:
		if peer.AccessHash == 0 {
			return nil, fmt.Errorf("missing access hash for user peer %s", peer.Key())
		}
		return &tg.InputPeerUser{UserID: peer.ID, AccessHash: peer.AccessHash}, nil
	case entity.PeerKindChat:
		return &tg.InputPeerChat{ChatID: peer.ID}, nil
	case entity.PeerKindChannel:
		if peer.AccessHash == 0 {
			return nil, fmt.Errorf("missing access hash for channel peer %s", peer.Key())
		}
		return &tg.InputPeerChannel{ChannelID: peer.ID, AccessHash: peer.AccessHash}, nil
	default:
		return nil, fmt.Errorf("unsupported peer kind %q", peer.Kind)
	}
}

func inputPeerKey(peer tg.InputPeerClass) string {
	switch peer.(type) {
	case *tg.InputPeerSelf:
		return "self"
	}
	ref, ok := peerRefFromInputPeer(peer, 0)
	if !ok {
		return ""
	}
	return ref.Key()
}

func peerRefFromPeer(peer tg.PeerClass, userMap map[int64]*tg.User, channelMap map[int64]*tg.Channel) (entity.PeerRef, bool) {
	switch p := peer.(type) {
	case *tg.PeerUser:
		user, ok := userMap[p.UserID]
		if !ok {
			return entity.PeerRef{}, false
		}
		return entity.PeerRef{Kind: entity.PeerKindUser, ID: user.ID, AccessHash: user.AccessHash}, true
	case *tg.PeerChat:
		return entity.PeerRef{Kind: entity.PeerKindChat, ID: p.ChatID}, true
	case *tg.PeerChannel:
		channel, ok := channelMap[p.ChannelID]
		if !ok {
			return entity.PeerRef{}, false
		}
		return entity.PeerRef{Kind: entity.PeerKindChannel, ID: channel.ID, AccessHash: channel.AccessHash}, true
	default:
		return entity.PeerRef{}, false
	}
}

func peerKey(peer tg.PeerClass) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return entity.PeerRef{Kind: entity.PeerKindUser, ID: p.UserID}.Key()
	case *tg.PeerChat:
		return entity.PeerRef{Kind: entity.PeerKindChat, ID: p.ChatID}.Key()
	case *tg.PeerChannel:
		return entity.PeerRef{Kind: entity.PeerKindChannel, ID: p.ChannelID}.Key()
	default:
		return ""
	}
}

func inputPeerFromDialogPeer(peer tg.PeerClass, userMap map[int64]*tg.User, channelMap map[int64]*tg.Channel) tg.InputPeerClass {
	if userPeer, ok := peer.(*tg.PeerUser); ok {
		if user, ok := userMap[userPeer.UserID]; ok && user.Self {
			return &tg.InputPeerSelf{}
		}
	}

	ref, ok := peerRefFromPeer(peer, userMap, channelMap)
	if !ok {
		return &tg.InputPeerEmpty{}
	}
	inputPeer, err := inputPeerFromPeerRef(ref)
	if err != nil {
		return &tg.InputPeerEmpty{}
	}
	return inputPeer
}
