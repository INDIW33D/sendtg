package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"sendtg/internal/domain/entity"
	"sendtg/internal/infrastructure/telegram"
)

// FileInfo is an alias for display purposes
type FileInfo = entity.FileInfo

// App represents the TUI application
type App struct {
	model    *Model
	program  *tea.Program
	useCases *UseCases
	client   *telegram.Client
}

// NewApp creates a new TUI application
func NewApp(useCases *UseCases, client *telegram.Client, authorizer *telegram.ManualAuthorizer, filePath string, fileInfo *entity.FileInfo) *App {
	model := NewModel(useCases, client, authorizer, filePath, fileInfo)

	return &App{
		model:    model,
		useCases: useCases,
		client:   client,
	}
}

// NewAppWithFileInfo creates a new TUI application with file info
func NewAppWithFileInfo(useCases *UseCases, client *telegram.Client, authorizer *telegram.ManualAuthorizer, filePath, fileName string, fileSize int64) *App {
	fileInfo := &entity.FileInfo{
		Path: filePath,
		Name: fileName,
		Size: fileSize,
	}
	return NewApp(useCases, client, authorizer, filePath, fileInfo)
}

// Run starts the TUI application
func (a *App) Run() error {
	a.program = tea.NewProgram(a.model, tea.WithAltScreen())
	finalModel, err := a.program.Run()
	if err != nil {
		return err
	}

	// Print final result to console after TUI closes
	if m, ok := finalModel.(*Model); ok {
		if m.state.Success {
			fmt.Println(SuccessStyle.Render("✓ " + m.state.ResultMsg))
		} else if m.state.ErrorMsg != "" && !m.quitting {
			fmt.Println(ErrorStyle.Render("✗ Error: " + m.state.ErrorMsg))
		}
	}

	return nil
}

// SendAuthState sends an auth state update to the program
func (a *App) SendAuthState(state entity.AuthState) {
	if a.program != nil {
		a.program.Send(AuthStateMsg(state))
	}
}
