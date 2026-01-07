package telegram

import (
	"sendtg/internal/domain/entity"
)

// AuthRepository implements the AuthRepository interface
type AuthRepository struct {
	client     *Client
	authorizer *ManualAuthorizer
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(c *Client, authorizer *ManualAuthorizer) *AuthRepository {
	return &AuthRepository{
		client:     c,
		authorizer: authorizer,
	}
}

// GetAuthState returns the current authentication state
func (r *AuthRepository) GetAuthState() (entity.AuthState, error) {
	return r.client.GetAuthState()
}

// SendPhoneNumber sends the phone number for authentication
func (r *AuthRepository) SendPhoneNumber(phone string) error {
	r.authorizer.SendPhoneNumber(phone)
	return nil
}

// SendCode sends the verification code
func (r *AuthRepository) SendCode(code string) error {
	r.authorizer.SendCode(code)
	return nil
}

// SendPassword sends the 2FA password
func (r *AuthRepository) SendPassword(password string) error {
	r.authorizer.SendPassword(password)
	return nil
}

// Close closes the client connection
func (r *AuthRepository) Close() error {
	return r.client.Close()
}
