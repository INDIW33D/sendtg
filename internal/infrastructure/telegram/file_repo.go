package telegram

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
)

// FileRepository implements the FileRepository interface
type FileRepository struct {
	client        *Client
	progressChan  chan entity.UploadProgress
	progressMutex sync.Mutex
}

// NewFileRepository creates a new file repository
func NewFileRepository(c *Client) *FileRepository {
	return &FileRepository{
		client: c,
	}
}

// SetProgressChan sets the channel for progress updates
func (r *FileRepository) SetProgressChan(ch chan entity.UploadProgress) {
	r.progressMutex.Lock()
	defer r.progressMutex.Unlock()
	r.progressChan = ch
}

// uploadProgress implements uploader.Progress interface
type uploadProgress struct {
	progressChan chan entity.UploadProgress
	total        int64
	startTime    time.Time
	lastUpdate   time.Time
}

func newUploadProgress(progressChan chan entity.UploadProgress, total int64) *uploadProgress {
	now := time.Now()
	return &uploadProgress{
		progressChan: progressChan,
		total:        total,
		startTime:    now,
		lastUpdate:   now,
	}
}

// Chunk implements uploader.Progress interface
func (p *uploadProgress) Chunk(ctx context.Context, state uploader.ProgressState) error {
	now := time.Now()
	p.lastUpdate = now

	elapsed := now.Sub(p.startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(state.Uploaded) / elapsed
	}

	percent := float64(state.Uploaded) / float64(p.total) * 100
	if percent > 100 {
		percent = 100
	}

	if p.progressChan != nil {
		select {
		case p.progressChan <- entity.UploadProgress{
			Uploaded: state.Uploaded,
			Total:    p.total,
			Speed:    speed,
			Percent:  percent,
		}:
		default:
			// Don't block if channel is full
		}
	}

	return nil
}

// SendFile sends a file to a specific chat
func (r *FileRepository) SendFile(chatID int64, filePath string) error {
	api := r.client.GetAPI()
	if api == nil {
		return fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()

	// Get the absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get file info for progress tracking
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Get progress channel
	r.progressMutex.Lock()
	progressChan := r.progressChan
	r.progressMutex.Unlock()

	// Create uploader with progress callback
	progress := newUploadProgress(progressChan, fileInfo.Size())
	u := uploader.NewUploader(api).WithProgress(progress)

	// Upload file
	uploaded, err := u.FromPath(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Resolve the peer - we need to get access hash for users
	peer, err := r.resolvePeer(chatID)
	if err != nil {
		return fmt.Errorf("failed to resolve peer: %w", err)
	}

	// Create document
	doc := &tg.InputMediaUploadedDocument{
		File:     uploaded,
		MimeType: getMimeType(fileInfo.Name()),
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{
				FileName: fileInfo.Name(),
			},
		},
	}

	// Send message
	_, err = api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    doc,
		Message:  "",
		RandomID: generateRandomID(),
	})
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	return nil
}

// resolvePeer resolves a chat ID to an InputPeer with access hash
func (r *FileRepository) resolvePeer(chatID int64) (tg.InputPeerClass, error) {
	api := r.client.GetAPI()
	ctx := r.client.GetContext()

	// First try to get the user from dialogs
	req := &tg.MessagesGetDialogsRequest{
		OffsetDate: 0,
		OffsetID:   0,
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
		Hash:       0,
	}

	result, err := api.MessagesGetDialogs(ctx, req)
	if err != nil {
		return &tg.InputPeerUser{UserID: chatID}, nil
	}

	switch res := result.(type) {
	case *tg.MessagesDialogs:
		return r.findPeerInDialogs(chatID, res.Users, res.Chats)
	case *tg.MessagesDialogsSlice:
		return r.findPeerInDialogs(chatID, res.Users, res.Chats)
	}

	return &tg.InputPeerUser{UserID: chatID}, nil
}

// findPeerInDialogs finds the peer with access hash from dialogs
func (r *FileRepository) findPeerInDialogs(chatID int64, users []tg.UserClass, chats []tg.ChatClass) (tg.InputPeerClass, error) {
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			if user.ID == chatID {
				return &tg.InputPeerUser{
					UserID:     user.ID,
					AccessHash: user.AccessHash,
				}, nil
			}
		}
	}

	for _, c := range chats {
		switch chat := c.(type) {
		case *tg.Chat:
			if chat.ID == chatID {
				return &tg.InputPeerChat{ChatID: chat.ID}, nil
			}
		case *tg.Channel:
			if chat.ID == chatID {
				return &tg.InputPeerChannel{
					ChannelID:  chat.ID,
					AccessHash: chat.AccessHash,
				}, nil
			}
		}
	}

	return &tg.InputPeerUser{UserID: chatID}, nil
}

// getMimeType returns a MIME type based on file extension
func getMimeType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/x-rar-compressed"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".7z":
		return "application/x-7z-compressed"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}

// generateRandomID generates a random ID for message sending
func generateRandomID() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}
