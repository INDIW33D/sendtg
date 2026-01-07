package entity

// AuthState represents the current authentication state
type AuthState int

const (
	AuthStateUnknown AuthState = iota
	AuthStateWaitPhoneNumber
	AuthStateWaitCode
	AuthStateWaitPassword
	AuthStateReady
	AuthStateClosed
)

// String returns a human-readable name for the auth state
func (s AuthState) String() string {
	switch s {
	case AuthStateWaitPhoneNumber:
		return "WaitPhoneNumber"
	case AuthStateWaitCode:
		return "WaitCode"
	case AuthStateWaitPassword:
		return "WaitPassword"
	case AuthStateReady:
		return "Ready"
	case AuthStateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}
