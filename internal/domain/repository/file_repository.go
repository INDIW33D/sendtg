package repository

import (
	"context"

	"sendtg/internal/domain/entity"
)

// FileRepository defines methods for file operations
type FileRepository interface {
	// SendFile sends a file to a specific chat
	SendFile(ctx context.Context, chat entity.Chat, filePath string) error

	// SetProgressChan sets the channel for progress updates
	SetProgressChan(ch chan entity.UploadProgress)
}
