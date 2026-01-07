package ui

import (
	"sendtg/internal/domain/entity"
	"sendtg/internal/usecase/auth"
	"sendtg/internal/usecase/chat"
	"sendtg/internal/usecase/file"
)

// Screen represents different application screens
type Screen int

const (
	ScreenLoading Screen = iota
	ScreenAuth
	ScreenMain // Main screen with folder tabs and chat list
	ScreenSending
	ScreenResult
)

// AuthStep represents the current authentication step
type AuthStep int

const (
	AuthStepPhone AuthStep = iota
	AuthStepCode
	AuthStepPassword
)

// AppState holds the application state
type AppState struct {
	// Current screen
	Screen Screen

	// Authentication state
	AuthStep  AuthStep
	AuthState entity.AuthState

	// File info
	FilePath string
	FileInfo *entity.FileInfo

	// Data
	Folders        []entity.Folder
	AllChats       []entity.Chat // All chats cached
	Chats          []entity.Chat // Filtered chats for current folder
	SelectedFolder int
	SelectedChat   int

	// Search
	SearchQuery string // Current search query for filtering chats

	// Result
	Success      bool
	ResultMsg    string
	ErrorMsg     string
	SelectedName string

	// Loading state
	Loading bool

	// Upload progress
	UploadProgress entity.UploadProgress
}

// UseCases holds all use cases
type UseCases struct {
	Auth *auth.UseCase
	Chat *chat.UseCase
	File *file.UseCase
}
