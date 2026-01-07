package repository

import "sendtg/internal/domain/entity"

// ChatRepository defines methods for chat operations
type ChatRepository interface {
	// GetFolders returns the list of chat folders
	GetFolders() ([]entity.Folder, error)

	// GetChatsByFolder returns chats in a specific folder
	GetChatsByFolder(folderID int32) ([]entity.Chat, error)

	// GetAllChats returns all chats (without folder filter)
	GetAllChats() ([]entity.Chat, error)

	// GetCachedChats returns chats from local cache (instant)
	GetCachedChats() []entity.Chat

	// GetCachedFolders returns folders from local cache (instant)
	GetCachedFolders() []entity.Folder

	// HasCachedData returns true if there's cached data available
	HasCachedData() bool

	// GetChatsFirstPage returns first page of chats quickly
	GetChatsFirstPage(limit int) ([]entity.Chat, error)

	// UpdateFoldersCache updates the folders cache
	UpdateFoldersCache(folders []entity.Folder)
}
