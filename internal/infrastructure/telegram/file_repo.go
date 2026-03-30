package telegram

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

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

	percent := 100.0
	if p.total > 0 {
		percent = float64(state.Uploaded) / float64(p.total) * 100
		if percent > 100 {
			percent = 100
		}
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

// SendFile sends a file to a specific chat.
func (r *FileRepository) SendFile(ctx context.Context, chat entity.Chat, filePath string) error {
	api := r.client.GetAPI()
	if api == nil {
		return fmt.Errorf("client not initialized")
	}

	if chat.Peer.IsZero() {
		return fmt.Errorf("chat peer is not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, uploadOperationTimeout)
	defer cancel()

	peer, err := r.inputPeerForPeer(ctx, chat.Peer)
	if err != nil {
		return fmt.Errorf("failed to build input peer: %w", err)
	}

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
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("failed to upload file: %w", err)
	}

	err = r.sendUploadedFile(ctx, api, peer, uploaded, fileInfo.Name())
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		if !isRetryablePeerError(err) {
			return fmt.Errorf("failed to send file: %w", err)
		}

		freshPeerRef, refreshErr := r.refreshPeerFromDialogs(ctx, chat.Peer)
		if refreshErr != nil {
			return fmt.Errorf("failed to refresh peer after send error: %v (refresh failed: %w)", err, refreshErr)
		}

		peer, refreshErr = r.inputPeerForPeer(ctx, freshPeerRef)
		if refreshErr != nil {
			return fmt.Errorf("failed to rebuild refreshed input peer: %w", refreshErr)
		}

		if retryErr := r.sendUploadedFile(ctx, api, peer, uploaded, fileInfo.Name()); retryErr != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			return fmt.Errorf("failed to send file after peer refresh: %w", retryErr)
		}
	}

	return nil
}

func (r *FileRepository) sendUploadedFile(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, uploaded tg.InputFileClass, fileName string) error {
	doc := &tg.InputMediaUploadedDocument{
		File:     uploaded,
		MimeType: getMimeType(fileName),
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: fileName},
		},
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendMediaRPCTimeout)
	defer cancel()

	_, err := api.MessagesSendMedia(sendCtx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    doc,
		Message:  "",
		RandomID: generateRandomID(),
	})
	return err
}

func (r *FileRepository) refreshPeerFromDialogs(ctx context.Context, target entity.PeerRef) (entity.PeerRef, error) {
	api := r.client.GetAPI()
	if api == nil {
		return entity.PeerRef{}, fmt.Errorf("client not initialized")
	}

	targetKey := target.Key()
	for _, peerFolderID := range []int32{mainPeerFolderID, archivePeerFolderID} {
		offsetDate := 0
		offsetID := 0
		var offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}
		limit := 100
		loadedDialogs := 0

		for {
			req := &tg.MessagesGetDialogsRequest{
				OffsetDate: offsetDate,
				OffsetID:   offsetID,
				OffsetPeer: offsetPeer,
				Limit:      limit,
				Hash:       0,
			}
			if offsetDate != 0 || offsetID != 0 || inputPeerKey(offsetPeer) != "" {
				req.ExcludePinned = true
			}
			if peerFolderID == archivePeerFolderID {
				req.FolderID = int(archivePeerFolderID)
			}

			dialogsCtx, cancel := context.WithTimeout(ctx, dialogsRPCTimeout)
			result, err := api.MessagesGetDialogs(dialogsCtx, req)
			cancel()
			if err != nil {
				return entity.PeerRef{}, err
			}

			var dialogs []tg.DialogClass
			var messages []tg.MessageClass
			var chats []tg.ChatClass
			var users []tg.UserClass
			var totalCount int

			switch res := result.(type) {
			case *tg.MessagesDialogs:
				dialogs = res.Dialogs
				messages = res.Messages
				chats = res.Chats
				users = res.Users
				totalCount = len(dialogs)
			case *tg.MessagesDialogsSlice:
				dialogs = res.Dialogs
				messages = res.Messages
				chats = res.Chats
				users = res.Users
				totalCount = res.Count
			case *tg.MessagesDialogsNotModified:
				break
			}

			userMap := buildUserMap(users)
			channelMap := buildChannelMap(chats)
			for _, dialogClass := range dialogs {
				dialog, ok := dialogClass.(*tg.Dialog)
				if !ok || peerKey(dialog.Peer) != targetKey {
					continue
				}
				if ref, ok := peerRefFromPeer(dialog.Peer, userMap, channelMap); ok {
					return ref, nil
				}
			}
			loadedDialogs += len(dialogs)

			if len(dialogs) == 0 || len(dialogs) < limit || loadedDialogs >= totalCount {
				break
			}

			nextOffsetDate, nextOffsetID, nextOffsetPeer, ok := nextDialogsPageOffset(dialogs, messages, userMap, channelMap)
			if !ok {
				break
			}
			if nextOffsetDate == offsetDate && nextOffsetID == offsetID && inputPeerKey(nextOffsetPeer) == inputPeerKey(offsetPeer) {
				break
			}

			offsetDate = nextOffsetDate
			offsetID = nextOffsetID
			offsetPeer = nextOffsetPeer
		}
	}

	return entity.PeerRef{}, fmt.Errorf("peer %s not found in dialogs", targetKey)
}

func (r *FileRepository) inputPeerForPeer(ctx context.Context, peer entity.PeerRef) (tg.InputPeerClass, error) {
	if peer.Kind == entity.PeerKindUser {
		selfCtx, cancel := context.WithTimeout(ctx, profileRPCTimeout)
		defer cancel()

		if selfID, err := r.client.SelfID(selfCtx); err == nil && selfID == peer.ID {
			return &tg.InputPeerSelf{}, nil
		}
	}

	return inputPeerFromPeerRef(peer)
}

func isRetryablePeerError(err error) bool {
	return tgerr.Is(err, "PEER_ID_INVALID", "CHANNEL_INVALID", "USER_ID_INVALID")
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
