package repository

import "sendtg/internal/domain/entity"

// AuthRepository defines methods for authentication operations
type AuthRepository interface {
	// GetAuthState returns the current authentication state
	GetAuthState() (entity.AuthState, error)

	// SendPhoneNumber sends the phone number for authentication
	SendPhoneNumber(phone string) error

	// SendCode sends the verification code
	SendCode(code string) error

	// SendPassword sends the 2FA password
	SendPassword(password string) error

	// Close closes the client connection
	Close() error
}
