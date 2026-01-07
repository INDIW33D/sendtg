package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sendtg/internal/domain/entity"
)

// DialogCache manages local caching of dialogs
type DialogCache struct {
	mu       sync.RWMutex
	cacheDir string
	data     *CacheData
}

// CacheData represents the cached data structure
type CacheData struct {
	Chats       []entity.Chat   `json:"chats"`
	Folders     []entity.Folder `json:"folders"`
	DialogsHash int64           `json:"dialogs_hash"`
	LastUpdate  time.Time       `json:"last_update"`
}

// NewDialogCache creates a new dialog cache
func NewDialogCache(cacheDir string) (*DialogCache, error) {
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, err
	}

	cache := &DialogCache{
		cacheDir: cacheDir,
		data:     &CacheData{},
	}

	// Try to load existing cache
	_ = cache.load()

	return cache, nil
}

// cacheFilePath returns the path to the cache file
func (c *DialogCache) cacheFilePath() string {
	return filepath.Join(c.cacheDir, "dialogs_cache.json")
}

// load loads the cache from disk
func (c *DialogCache) load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.cacheFilePath())
	if err != nil {
		return err
	}

	var cacheData CacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return err
	}

	c.data = &cacheData
	return nil
}

// save saves the cache to disk
func (c *DialogCache) save() error {
	data, err := json.Marshal(c.data)
	if err != nil {
		return err
	}

	return os.WriteFile(c.cacheFilePath(), data, 0600)
}

// GetChats returns cached chats
func (c *DialogCache) GetChats() []entity.Chat {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return nil
	}

	// Return a copy to prevent race conditions
	chats := make([]entity.Chat, len(c.data.Chats))
	copy(chats, c.data.Chats)
	return chats
}

// GetFolders returns cached folders
func (c *DialogCache) GetFolders() []entity.Folder {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return nil
	}

	folders := make([]entity.Folder, len(c.data.Folders))
	copy(folders, c.data.Folders)
	return folders
}

// SetChats updates the cached chats
func (c *DialogCache) SetChats(chats []entity.Chat) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data.Chats = chats
	c.data.LastUpdate = time.Now()
	c.data.DialogsHash = computeDialogsHash(chats)

	return c.save()
}

// SetFolders updates the cached folders
func (c *DialogCache) SetFolders(folders []entity.Folder) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data.Folders = folders
	c.data.LastUpdate = time.Now()

	return c.save()
}

// GetDialogsHash returns the hash for cache validation
func (c *DialogCache) GetDialogsHash() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return 0
	}
	return c.data.DialogsHash
}

// GetLastUpdate returns when the cache was last updated
func (c *DialogCache) GetLastUpdate() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return time.Time{}
	}
	return c.data.LastUpdate
}

// IsValid checks if cache is still valid (not older than maxAge)
func (c *DialogCache) IsValid(maxAge time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil || len(c.data.Chats) == 0 {
		return false
	}

	return time.Since(c.data.LastUpdate) < maxAge
}

// HasData returns true if cache has any data
func (c *DialogCache) HasData() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.data != nil && len(c.data.Chats) > 0
}

// Clear clears the cache
func (c *DialogCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = &CacheData{}
	return os.Remove(c.cacheFilePath())
}

// computeDialogsHash computes hash for dialogs according to Telegram algorithm
// Hash = XOR of (peer_id * 0x20000000 + top_message_id) for all dialogs
func computeDialogsHash(chats []entity.Chat) int64 {
	if len(chats) == 0 {
		return 0
	}

	var hash int64
	for _, chat := range chats {
		// Use chat ID and last message date as a simple hash component
		// This is a simplified version - Telegram uses top_message_id
		component := chat.ID*0x20000000 + int64(chat.LastMessageDate)
		hash ^= component
	}
	return hash
}
