package main

import (
	"fmt"
	"os"
	"path/filepath"

	"sendtg/internal/config"
	"sendtg/internal/infrastructure/cache"
	"sendtg/internal/infrastructure/telegram"
	"sendtg/internal/ui"
	"sendtg/internal/usecase/auth"
	"sendtg/internal/usecase/chat"
	"sendtg/internal/usecase/file"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Check command line arguments
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: sendtg <filename>")
	}

	filePath := os.Args[1]

	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Validate file exists
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", absPath)
		}
		return fmt.Errorf("cannot access file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	// Load embedded configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Create Telegram client config
	tgConfig := telegram.DefaultConfig()
	tgConfig.APIID = cfg.APIID
	tgConfig.APIHash = cfg.APIHash

	// Create dialog cache (cross-platform cache directory)
	// Linux: ~/.cache/sendtg
	// macOS: ~/Library/Caches/sendtg
	// Windows: %LOCALAPPDATA%\sendtg
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = filepath.Join(os.TempDir(), "sendtg")
	} else {
		cacheDir = filepath.Join(cacheDir, "sendtg")
	}
	dialogCache, err := cache.NewDialogCache(cacheDir)
	if err != nil {
		// Cache is optional, continue without it
		dialogCache = nil
	}

	// Create client with manual auth
	client, err := telegram.NewClientWithManualAuth(tgConfig)
	if err != nil {
		return fmt.Errorf("failed to create Telegram client: %w", err)
	}

	// Create manual authorizer
	authorizer := telegram.NewManualAuthorizer(client, tgConfig)

	// Create repositories
	authRepo := telegram.NewAuthRepository(client, authorizer)
	chatRepo := telegram.NewChatRepository(client, dialogCache)
	fileRepo := telegram.NewFileRepository(client)

	// Create use cases
	useCases := &ui.UseCases{
		Auth: auth.NewUseCase(authRepo),
		Chat: chat.NewUseCase(chatRepo),
		File: file.NewUseCase(fileRepo),
	}

	// Start client in background
	go func() {
		if err := client.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start Telegram client: %v\n", err)
		}
	}()

	// Create and run TUI app
	app := ui.NewAppWithFileInfo(useCases, client, authorizer, absPath, fileInfo.Name(), fileInfo.Size())
	if err := app.Run(); err != nil {
		client.Close()
		return err
	}

	// Cleanup
	return client.Close()
}
