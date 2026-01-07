package file

import (
	"fmt"
	"os"

	"sendtg/internal/domain/entity"
	"sendtg/internal/domain/repository"
)

// UseCase handles file sending operations
type UseCase struct {
	fileRepo repository.FileRepository
}

// NewUseCase creates a new file use case
func NewUseCase(fileRepo repository.FileRepository) *UseCase {
	return &UseCase{
		fileRepo: fileRepo,
	}
}

// ValidateFile validates that the file exists and is readable
func (uc *UseCase) ValidateFile(filePath string) (*entity.FileInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", filePath)
		}
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	return &entity.FileInfo{
		Path: filePath,
		Name: info.Name(),
		Size: info.Size(),
	}, nil
}

// SendFile sends a file to a specific chat
func (uc *UseCase) SendFile(chatID int64, filePath string) error {
	return uc.fileRepo.SendFile(chatID, filePath)
}

// SetProgressChan sets the channel for progress updates
func (uc *UseCase) SetProgressChan(ch chan entity.UploadProgress) {
	uc.fileRepo.SetProgressChan(ch)
}
