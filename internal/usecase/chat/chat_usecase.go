package chat

import (
	"sendtg/internal/domain/entity"
	"sendtg/internal/domain/repository"
)

// UseCase handles chat operations
type UseCase struct {
	chatRepo repository.ChatRepository
}

// NewUseCase creates a new chat use case
func NewUseCase(chatRepo repository.ChatRepository) *UseCase {
	return &UseCase{
		chatRepo: chatRepo,
	}
}

// GetFolders returns all chat folders
func (uc *UseCase) GetFolders() ([]entity.Folder, error) {
	folders, err := uc.chatRepo.GetFolders()
	if err != nil {
		return nil, err
	}
	// Update cache
	uc.chatRepo.UpdateFoldersCache(folders)
	return folders, nil
}

// GetChatsByFolder returns chats in a specific folder
func (uc *UseCase) GetChatsByFolder(folderID int32) ([]entity.Chat, error) {
	return uc.chatRepo.GetChatsByFolder(folderID)
}

// GetAllChats returns all chats
func (uc *UseCase) GetAllChats() ([]entity.Chat, error) {
	return uc.chatRepo.GetAllChats()
}

// GetCachedChats returns chats from local cache (instant)
func (uc *UseCase) GetCachedChats() []entity.Chat {
	return uc.chatRepo.GetCachedChats()
}

// GetCachedFolders returns folders from local cache (instant)
func (uc *UseCase) GetCachedFolders() []entity.Folder {
	return uc.chatRepo.GetCachedFolders()
}

// HasCachedData returns true if there's cached data available
func (uc *UseCase) HasCachedData() bool {
	return uc.chatRepo.HasCachedData()
}

// GetChatsFirstPage returns first page of chats quickly
func (uc *UseCase) GetChatsFirstPage(limit int) ([]entity.Chat, error) {
	return uc.chatRepo.GetChatsFirstPage(limit)
}
