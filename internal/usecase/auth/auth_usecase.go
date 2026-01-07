package auth

import (
	"sendtg/internal/domain/entity"
	"sendtg/internal/domain/repository"
)

// UseCase handles authentication operations
type UseCase struct {
	authRepo repository.AuthRepository
}

// NewUseCase creates a new auth use case
func NewUseCase(authRepo repository.AuthRepository) *UseCase {
	return &UseCase{
		authRepo: authRepo,
	}
}

// GetAuthState returns the current authentication state
func (uc *UseCase) GetAuthState() (entity.AuthState, error) {
	return uc.authRepo.GetAuthState()
}

// SendPhoneNumber sends the phone number for authentication
func (uc *UseCase) SendPhoneNumber(phone string) error {
	return uc.authRepo.SendPhoneNumber(phone)
}

// SendCode sends the verification code
func (uc *UseCase) SendCode(code string) error {
	return uc.authRepo.SendCode(code)
}

// SendPassword sends the 2FA password
func (uc *UseCase) SendPassword(password string) error {
	return uc.authRepo.SendPassword(password)
}

// IsAuthorized checks if the user is authorized
func (uc *UseCase) IsAuthorized() (bool, error) {
	state, err := uc.authRepo.GetAuthState()
	if err != nil {
		return false, err
	}
	return state == entity.AuthStateReady, nil
}
