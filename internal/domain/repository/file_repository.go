package repository

import "sendtg/internal/domain/entity"

// FileRepository defines methods for file operations
type FileRepository interface {
	// SendFile sends a file to a specific chat
	SendFile(chatID int64, filePath string) error

	// SetProgressChan sets the channel for progress updates
	SetProgressChan(ch chan entity.UploadProgress)
}
