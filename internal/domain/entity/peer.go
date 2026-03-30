package entity

import "fmt"

// PeerKind identifies the Telegram peer type.
type PeerKind string

const (
	PeerKindUser    PeerKind = "user"
	PeerKindChat    PeerKind = "chat"
	PeerKindChannel PeerKind = "channel"
)

// PeerRef stores the typed Telegram peer identity.
type PeerRef struct {
	Kind       PeerKind
	ID         int64
	AccessHash int64
}

// IsZero returns true when the peer is not initialized.
func (p PeerRef) IsZero() bool {
	return p.Kind == "" || p.ID == 0
}

// Key returns a stable typed key for the peer.
func (p PeerRef) Key() string {
	if p.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s:%d", p.Kind, p.ID)
}

// Matches reports whether both peers point to the same Telegram entity.
func (p PeerRef) Matches(other PeerRef) bool {
	return p.Key() != "" && p.Key() == other.Key()
}
