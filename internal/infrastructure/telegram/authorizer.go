package telegram

import (
	"sendtg/internal/domain/entity"
)

// ManualAuthorizer wraps the auth flow for TUI
type ManualAuthorizer struct {
	client   *Client
	authFlow *AuthFlow
}

// NewManualAuthorizer creates a new manual authorizer
func NewManualAuthorizer(c *Client, config *Config) *ManualAuthorizer {
	return &ManualAuthorizer{
		client:   c,
		authFlow: c.GetAuthFlow(),
	}
}

// SendPhoneNumber sends the phone number for authentication
func (a *ManualAuthorizer) SendPhoneNumber(phone string) {
	a.authFlow.SendPhoneNumber(phone)
}

// SendCode sends the verification code
func (a *ManualAuthorizer) SendCode(code string) {
	a.authFlow.SendCode(code)
}

// SendPassword sends the 2FA password
func (a *ManualAuthorizer) SendPassword(password string) {
	a.authFlow.SendPassword(password)
}

// GetAuthState returns the current auth state
func (a *ManualAuthorizer) GetAuthState() (entity.AuthState, error) {
	return a.client.GetAuthState()
}

// AuthStateChan returns the auth state channel
func (a *ManualAuthorizer) AuthStateChan() <-chan entity.AuthState {
	return a.client.AuthStateChan()
}
