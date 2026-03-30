package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sendtg/internal/domain/entity"
	"sendtg/internal/infrastructure/telegram"
)

// Messages for communication between components
type (
	// AuthStateMsg is sent when auth state changes
	AuthStateMsg entity.AuthState

	// FoldersLoadedMsg is sent when folders are loaded
	FoldersLoadedMsg struct {
		Folders   []entity.Folder
		Err       error
		FromCache bool
	}

	// FoldersRefreshedMsg is sent when folders are refreshed from server.
	FoldersRefreshedMsg struct {
		Folders []entity.Folder
		Err     error
	}

	// ChatsLoadedMsg is sent when chats are loaded
	ChatsLoadedMsg struct {
		Chats     []entity.Chat
		Err       error
		FromCache bool
	}

	// ChatsRefreshedMsg is sent when chats are refreshed from server
	ChatsRefreshedMsg struct {
		Chats []entity.Chat
		Err   error
	}

	// FileSentMsg is sent when file is sent
	FileSentMsg struct {
		Err error
	}

	// ErrorMsg is sent when an error occurs
	ErrorMsg struct {
		Err error
	}

	// AuthErrorMsg is sent when the auth flow fails.
	AuthErrorMsg struct {
		Err         error
		Recoverable bool
	}

	// InitCompleteMsg is sent when initialization is complete
	InitCompleteMsg struct {
		AuthState entity.AuthState
		Err       error
	}

	// UploadProgressMsg is sent when upload progress updates
	UploadProgressMsg entity.UploadProgress
)

const initialAuthEventTimeout = 45 * time.Second

// Model is the main application model
type Model struct {
	state      *AppState
	useCases   *UseCases
	client     *telegram.Client
	authorizer *telegram.ManualAuthorizer

	// UI components
	phoneInput    textinput.Model
	codeInput     textinput.Model
	passwordInput textinput.Model
	spinner       spinner.Model
	progressBar   progress.Model

	// Window size
	width  int
	height int

	// Quit flag
	quitting bool

	// Progress channel
	progressChan chan entity.UploadProgress

	// File sending
	fileSendResult chan error
	sendCancel     context.CancelFunc
}

// NewModel creates a new model
func NewModel(useCases *UseCases, client *telegram.Client, authorizer *telegram.ManualAuthorizer, filePath string, fileInfo *entity.FileInfo) *Model {
	// Phone input
	phoneInput := textinput.New()
	phoneInput.Placeholder = "+1234567890"
	phoneInput.Focus()
	phoneInput.CharLimit = 20
	phoneInput.Width = 30

	// Code input
	codeInput := textinput.New()
	codeInput.Placeholder = "12345"
	codeInput.CharLimit = 10
	codeInput.Width = 30

	// Password input
	passwordInput := textinput.New()
	passwordInput.Placeholder = "Your 2FA password"
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.CharLimit = 50
	passwordInput.Width = 30

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	// Progress bar
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 40

	// Progress channel
	progressChan := make(chan entity.UploadProgress, 100)

	return &Model{
		state: &AppState{
			Screen:   ScreenLoading,
			FilePath: filePath,
			FileInfo: fileInfo,
			Loading:  true,
		},
		useCases:       useCases,
		client:         client,
		authorizer:     authorizer,
		phoneInput:     phoneInput,
		codeInput:      codeInput,
		passwordInput:  passwordInput,
		spinner:        s,
		progressBar:    p,
		progressChan:   progressChan,
		fileSendResult: make(chan error, 1),
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.checkAuthState,
	)
}

// checkAuthState checks the current authentication state
func (m *Model) checkAuthState() tea.Msg {
	state, err := m.useCases.Auth.GetAuthState()
	if err != nil {
		return InitCompleteMsg{Err: err}
	}

	// If state is Unknown, wait for the first real state from channel
	if state == entity.AuthStateUnknown {
		select {
		case event := <-m.authorizer.EventChan():
			if event.Err != nil {
				return InitCompleteMsg{Err: event.Err}
			}
			state = event.State
		case <-time.After(initialAuthEventTimeout):
			return InitCompleteMsg{Err: fmt.Errorf("timed out waiting for Telegram auth state")}
		case <-m.client.GetContext().Done():
			return InitCompleteMsg{Err: m.client.GetContext().Err()}
		}
	}

	return InitCompleteMsg{AuthState: state}
}

// listenAuthEvent listens for auth state and error changes.
func (m *Model) listenAuthEvent() tea.Msg {
	select {
	case event := <-m.authorizer.EventChan():
		if event.Err != nil {
			return AuthErrorMsg{Err: event.Err, Recoverable: event.Recoverable}
		}
		return AuthStateMsg(event.State)
	case <-m.client.GetContext().Done():
		return AuthErrorMsg{Err: m.client.GetContext().Err()}
	}
}

// loadFolders loads the folders (tries cache first)
func (m *Model) loadFolders() tea.Msg {
	// Try to load from cache first for instant display
	cachedFolders := m.useCases.Chat.GetCachedFolders()
	if len(cachedFolders) > 0 {
		return FoldersLoadedMsg{Folders: cachedFolders, Err: nil, FromCache: true}
	}

	// Load from server
	folders, err := m.useCases.Chat.GetFolders()
	return FoldersLoadedMsg{Folders: folders, Err: err, FromCache: false}
}

// refreshFoldersFromServer loads fresh folders from server.
func (m *Model) refreshFoldersFromServer() tea.Msg {
	folders, err := m.useCases.Chat.GetFolders()
	return FoldersRefreshedMsg{Folders: folders, Err: err}
}

// loadAllChats loads all chats (tries cache first, then loads from server)
func (m *Model) loadAllChats() tea.Msg {
	// Try to load from cache first for instant display
	cachedChats := m.useCases.Chat.GetCachedChats()
	if len(cachedChats) > 0 {
		return ChatsLoadedMsg{Chats: cachedChats, Err: nil, FromCache: true}
	}

	// No cache - load from server
	chats, err := m.useCases.Chat.GetAllChats()
	return ChatsLoadedMsg{Chats: chats, Err: err, FromCache: false}
}

// refreshChatsFromServer loads fresh chats from server (called after cache is displayed)
func (m *Model) refreshChatsFromServer() tea.Msg {
	chats, err := m.useCases.Chat.GetAllChats()
	return ChatsRefreshedMsg{Chats: chats, Err: err}
}

// filterChatsForFolder filters cached chats for the current folder and search query
func (m *Model) filterChatsForFolder() {
	var baseChats []entity.Chat

	if m.state.SelectedFolder >= len(m.state.Folders) {
		baseChats = deduplicateChats(m.state.AllChats)
	} else {
		folder := m.state.Folders[m.state.SelectedFolder]

		// Built-in folders use the server-backed chat inventory directly.
		if folder.IsAllChats() || folder.IsArchive() {
			filtered := make([]entity.Chat, 0, len(m.state.AllChats))
			seenKeys := make(map[string]bool)
			for _, chat := range m.state.AllChats {
				key := chat.UniqueKey()
				if seenKeys[key] {
					continue
				}
				if folder.ContainsChat(chat) {
					filtered = append(filtered, chat)
					seenKeys[key] = true
				}
			}
			baseChats = filtered
		} else {
			// Filter chats based on folder settings
			filtered := make([]entity.Chat, 0)
			seenKeys := make(map[string]bool)
			for _, chat := range m.state.AllChats {
				key := chat.UniqueKey()
				if seenKeys[key] {
					continue
				}
				if folder.ContainsChat(chat) {
					filtered = append(filtered, chat)
					seenKeys[key] = true
				}
			}
			baseChats = filtered
		}
	}

	// Filter to only show chats where user can write
	baseChats = filterWritableChats(baseChats)

	// Apply search filter if there's a query
	if m.state.SearchQuery != "" {
		baseChats = filterChatsBySearch(baseChats, m.state.SearchQuery)
	}

	folder := entity.Folder{}
	if m.state.SelectedFolder < len(m.state.Folders) {
		folder = m.state.Folders[m.state.SelectedFolder]
	}

	m.state.Chats = sortChatsForFolder(baseChats, folder)
	m.state.SelectedChat = 0
}

func (m *Model) applyFolders(folders []entity.Folder) {
	selectedFolderKey := string(entity.FolderKindAll)
	if m.state.SelectedFolder >= 0 && m.state.SelectedFolder < len(m.state.Folders) {
		selectedFolderKey = m.state.Folders[m.state.SelectedFolder].Key()
	}

	m.state.Folders = folders
	m.state.SelectedFolder = 0
	for i, folder := range folders {
		if folder.Key() == selectedFolderKey {
			m.state.SelectedFolder = i
			break
		}
	}
}

// filterWritableChats filters to only include chats where user can send messages
func filterWritableChats(chats []entity.Chat) []entity.Chat {
	result := make([]entity.Chat, 0, len(chats))
	for _, chat := range chats {
		if chat.CanWrite {
			result = append(result, chat)
		}
	}
	return result
}

// filterChatsBySearch filters chats by search query (case-insensitive)
func filterChatsBySearch(chats []entity.Chat, query string) []entity.Chat {
	query = strings.ToLower(query)
	result := make([]entity.Chat, 0)
	for _, chat := range chats {
		if strings.Contains(strings.ToLower(chat.Title), query) {
			result = append(result, chat)
		}
	}
	return result
}

// deduplicateChats removes duplicate chats by UniqueKey (type + ID)
func deduplicateChats(chats []entity.Chat) []entity.Chat {
	seen := make(map[string]bool)
	result := make([]entity.Chat, 0, len(chats))
	for _, chat := range chats {
		key := chat.UniqueKey()
		if !seen[key] {
			seen[key] = true
			result = append(result, chat)
		}
	}
	return result
}

// sortChatsForFolder sorts chats: pinned first (using folder-specific pins), then others by original Telegram order
func sortChatsForFolder(chats []entity.Chat, folder entity.Folder) []entity.Chat {
	sorted := make([]entity.Chat, len(chats))
	copy(sorted, chats)

	sort.SliceStable(sorted, func(i, j int) bool {
		var iPinned, jPinned bool
		var iPinOrder, jPinOrder int

		// Built-in folders use the pinned order returned by Telegram for that peer folder.
		if folder.IsAllChats() || folder.IsArchive() {
			iPinned = sorted[i].IsPinned
			jPinned = sorted[j].IsPinned
			iPinOrder = sorted[i].PinOrder
			jPinOrder = sorted[j].PinOrder
		} else {
			// For other folders, use folder-specific pinned peers
			iPinned, iPinOrder = folder.IsPinnedInFolder(sorted[i].Peer)
			jPinned, jPinOrder = folder.IsPinnedInFolder(sorted[j].Peer)
		}

		// Pinned chats come first
		if iPinned && !jPinned {
			return true
		}
		if !iPinned && jPinned {
			return false
		}
		// Both pinned - sort by pin order
		if iPinned && jPinned {
			return iPinOrder < jPinOrder
		}
		// Both not pinned - prefer newer chats, then stable Telegram order.
		if sorted[i].LastMessageDate != sorted[j].LastMessageDate {
			return sorted[i].LastMessageDate > sorted[j].LastMessageDate
		}
		return sorted[i].Order < sorted[j].Order
	})

	return sorted
}

// startFileSend starts the file upload in a goroutine
func (m *Model) startFileSend() tea.Cmd {
	return func() tea.Msg {
		if m.state.SelectedChat >= len(m.state.Chats) {
			return FileSentMsg{Err: fmt.Errorf("invalid chat selection")}
		}
		chat := m.state.Chats[m.state.SelectedChat]
		sendCtx, cancel := context.WithCancel(m.client.GetContext())
		m.sendCancel = cancel

		// Set progress channel before sending
		m.useCases.File.SetProgressChan(m.progressChan)

		// Run upload in goroutine
		go func() {
			defer cancel()
			err := m.useCases.File.SendFile(sendCtx, chat, m.state.FilePath)
			m.fileSendResult <- err
		}()

		return nil
	}
}

// waitForFileSent waits for file send result
func (m *Model) waitForFileSent() tea.Msg {
	select {
	case err := <-m.fileSendResult:
		return FileSentMsg{Err: err}
	case <-time.After(100 * time.Millisecond):
		return nil // Continue waiting
	}
}

// listenProgress listens for upload progress updates using tick
func (m *Model) tickProgress() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		select {
		case progress := <-m.progressChan:
			return UploadProgressMsg(progress)
		default:
			return tickMsg{}
		}
	})
}

// tickMsg is sent periodically to check for progress
type tickMsg struct{}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle nil messages
	if msg == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tickMsg:
		// Continue ticking for progress while sending
		if m.state.Screen == ScreenSending {
			return m, tea.Batch(m.tickProgress(), m.waitForFileSent)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Ctrl+C always quits
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Esc on loading or result screens quits
			if m.state.Screen == ScreenResult || m.state.Screen == ScreenLoading {
				m.quitting = true
				return m, tea.Quit
			}
			// Esc on main screen is handled in updateMain (clears search or quits)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case InitCompleteMsg:
		m.state.Loading = false
		if msg.Err != nil {
			m.state.Screen = ScreenResult
			m.state.Success = false
			m.state.ErrorMsg = msg.Err.Error()
			return m, nil
		}
		m.state.AuthState = msg.AuthState
		model, cmd := m.handleAuthState(msg.AuthState)
		// Start listening for auth state changes after initial check.
		return model, tea.Batch(cmd, m.listenAuthEvent)

	case AuthStateMsg:
		newState := entity.AuthState(msg)
		// If we're already authorized and loading/showing main screen, ignore intermediate auth states
		if newState == entity.AuthStateReady && (m.state.Screen == ScreenLoading || m.state.Screen == ScreenMain) {
			return m, m.listenAuthEvent
		}
		// If we're in sending/result screen, don't interrupt
		if m.state.Screen == ScreenSending || m.state.Screen == ScreenResult {
			return m, m.listenAuthEvent
		}
		m.state.AuthState = newState
		model, cmd := m.handleAuthState(newState)
		return model, tea.Batch(cmd, m.listenAuthEvent)

	case AuthErrorMsg:
		m.state.Loading = false
		m.state.ErrorMsg = msg.Err.Error()
		if msg.Recoverable {
			if m.state.Screen != ScreenAuth {
				m.state.Screen = ScreenAuth
			}
			return m, m.listenAuthEvent
		}
		m.state.Success = false
		m.state.Screen = ScreenResult
		return m, nil

	case FoldersLoadedMsg:
		if msg.Err != nil {
			m.state.Loading = false
			m.state.Screen = ScreenResult
			m.state.Success = false
			m.state.ErrorMsg = fmt.Sprintf("Failed to load folders: %v", msg.Err)
			return m, nil
		}
		m.applyFolders(msg.Folders)
		// Load all chats once
		if msg.FromCache {
			return m, tea.Batch(m.loadAllChats, m.refreshFoldersFromServer)
		}
		return m, m.loadAllChats

	case FoldersRefreshedMsg:
		if msg.Err == nil {
			m.applyFolders(msg.Folders)
			if len(m.state.AllChats) > 0 {
				m.filterChatsForFolder()
			}
		}
		return m, nil

	case ChatsLoadedMsg:
		m.state.Loading = false
		if msg.Err != nil {
			m.state.Screen = ScreenResult
			m.state.Success = false
			m.state.ErrorMsg = fmt.Sprintf("Failed to load chats: %v", msg.Err)
			return m, nil
		}
		// Cache all chats
		m.state.AllChats = msg.Chats
		// Filter for current folder
		m.filterChatsForFolder()
		m.state.Screen = ScreenMain

		// If loaded from cache, refresh from server in background
		if msg.FromCache {
			return m, m.refreshChatsFromServer
		}
		return m, nil

	case ChatsRefreshedMsg:
		// Update chats with fresh data from server (silently, no UI change)
		if msg.Err == nil {
			m.state.AllChats = msg.Chats
			m.filterChatsForFolder()
		}
		return m, nil

	case FileSentMsg:
		if m.sendCancel != nil {
			m.sendCancel()
			m.sendCancel = nil
		}
		m.state.Loading = false
		m.state.Screen = ScreenResult
		if msg.Err != nil {
			m.state.Success = false
			if errors.Is(msg.Err, context.Canceled) {
				m.state.ErrorMsg = "Upload cancelled by user"
			} else {
				m.state.ErrorMsg = fmt.Sprintf("Failed to send file: %v", msg.Err)
			}
		} else {
			m.state.Success = true
			m.state.ErrorMsg = ""
			m.state.ResultMsg = fmt.Sprintf("File '%s' successfully sent to %s", m.state.FileInfo.Name, m.state.SelectedName)
		}
		return m, nil

	case UploadProgressMsg:
		m.state.UploadProgress = entity.UploadProgress(msg)
		// Continue ticking for progress
		if m.state.Screen == ScreenSending {
			return m, tea.Batch(m.tickProgress(), m.waitForFileSent)
		}
		return m, nil

	case progress.FrameMsg:
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd
	}

	// Handle screen-specific updates
	switch m.state.Screen {
	case ScreenAuth:
		return m.updateAuth(msg)
	case ScreenMain:
		return m.updateMain(msg)
	case ScreenSending:
		return m.updateSending(msg)
	case ScreenResult:
		return m.updateResult(msg)
	}

	return m, tea.Batch(cmds...)
}

// handleAuthState handles auth state changes
func (m *Model) handleAuthState(state entity.AuthState) (*Model, tea.Cmd) {
	m.state.ErrorMsg = ""
	m.state.Loading = false

	switch state {
	case entity.AuthStateReady:
		m.state.Screen = ScreenLoading
		m.state.Loading = true
		return m, m.loadFolders
	case entity.AuthStateWaitPhoneNumber:
		m.state.Screen = ScreenAuth
		m.state.AuthStep = AuthStepPhone
		m.phoneInput.Focus()
		return m, textinput.Blink
	case entity.AuthStateWaitCode:
		m.state.Screen = ScreenAuth
		m.state.AuthStep = AuthStepCode
		m.codeInput.Focus()
		return m, textinput.Blink
	case entity.AuthStateWaitPassword:
		m.state.Screen = ScreenAuth
		m.state.AuthStep = AuthStepPassword
		m.passwordInput.Focus()
		return m, textinput.Blink
	default:
		m.state.Screen = ScreenAuth
		m.state.AuthStep = AuthStepPhone
		m.phoneInput.Focus()
		return m, textinput.Blink
	}
}

// updateAuth handles auth screen updates
func (m *Model) updateAuth(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.state.ErrorMsg = ""
			switch m.state.AuthStep {
			case AuthStepPhone:
				phone := m.phoneInput.Value()
				if phone == "" {
					return m, nil
				}
				m.state.Loading = true
				m.authorizer.SendPhoneNumber(phone)
				return m, m.spinner.Tick
			case AuthStepCode:
				code := m.codeInput.Value()
				if code == "" {
					return m, nil
				}
				m.state.Loading = true
				m.authorizer.SendCode(code)
				return m, m.spinner.Tick
			case AuthStepPassword:
				password := m.passwordInput.Value()
				if password == "" {
					return m, nil
				}
				m.state.Loading = true
				m.authorizer.SendPassword(password)
				return m, m.spinner.Tick
			}
		}
	}

	// Update the appropriate input
	switch m.state.AuthStep {
	case AuthStepPhone:
		m.phoneInput, cmd = m.phoneInput.Update(msg)
	case AuthStepCode:
		m.codeInput, cmd = m.codeInput.Update(msg)
	case AuthStepPassword:
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}

	return m, cmd
}

// updateMain handles main screen updates (tabs + chat list)
func (m *Model) updateMain(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		// Clear search with Escape
		case "esc":
			if m.state.SearchQuery != "" {
				m.state.SearchQuery = ""
				m.filterChatsForFolder()
				return m, nil
			}
			// If no search query, quit
			m.quitting = true
			return m, tea.Quit
		// Delete last character from search
		case "backspace":
			if len(m.state.SearchQuery) > 0 {
				// Delete last rune, not last byte (for proper UTF-8 support)
				runes := []rune(m.state.SearchQuery)
				m.state.SearchQuery = string(runes[:len(runes)-1])
				m.filterChatsForFolder()
			}
			return m, nil
		// Navigate tabs with left/right or Tab/Shift+Tab
		case "left", "shift+tab":
			if m.state.SearchQuery == "" {
				if m.state.SelectedFolder > 0 {
					m.state.SelectedFolder--
					m.filterChatsForFolder()
				}
			}
		case "right", "tab":
			if m.state.SearchQuery == "" {
				if m.state.SelectedFolder < len(m.state.Folders)-1 {
					m.state.SelectedFolder++
					m.filterChatsForFolder()
				}
			}
		// Navigate chats with up/down
		case "up":
			if m.state.SelectedChat > 0 {
				m.state.SelectedChat--
			}
		case "down":
			if m.state.SelectedChat < len(m.state.Chats)-1 {
				m.state.SelectedChat++
			}
		// Select chat and send file
		case "enter":
			if len(m.state.Chats) == 0 {
				return m, nil
			}
			m.state.SelectedName = m.state.Chats[m.state.SelectedChat].DisplayName()
			m.state.Loading = true
			m.state.ErrorMsg = ""
			m.state.Screen = ScreenSending
			m.state.UploadProgress = entity.UploadProgress{Total: m.state.FileInfo.Size, Percent: initialUploadPercent(m.state.FileInfo.Size)}
			return m, tea.Batch(m.startFileSend(), m.tickProgress(), m.waitForFileSent)
		default:
			// Handle regular character input for search
			if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				// ASCII printable characters
				m.state.SearchQuery += key
				m.filterChatsForFolder()
				return m, nil
			}
			// Handle Unicode characters (like Cyrillic)
			if len(key) > 1 && !strings.HasPrefix(key, "ctrl+") && !strings.HasPrefix(key, "alt+") {
				m.state.SearchQuery += key
				m.filterChatsForFolder()
				return m, nil
			}
		}
	}
	return m, nil
}

// updateResult handles result screen updates
func (m *Model) updateResult(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the UI
func (m *Model) View() string {
	if m.quitting {
		if m.state.Success {
			return SuccessStyle.Render("✓ "+m.state.ResultMsg) + "\n"
		}
		if m.state.ErrorMsg != "" {
			return ErrorStyle.Render("✗ Error: "+m.state.ErrorMsg) + "\n"
		}
		return ""
	}

	var s strings.Builder

	// Header
	s.WriteString(TitleStyle.Render(titleText()))
	s.WriteString("\n")

	// File info
	if m.state.FileInfo != nil {
		s.WriteString(FileInfoStyle.Render(fmt.Sprintf("File: %s (%s)", m.state.FileInfo.Name, formatBytes(m.state.FileInfo.Size))))
		s.WriteString("\n\n")
	}

	// Screen content
	switch m.state.Screen {
	case ScreenLoading:
		s.WriteString(m.viewLoading())
	case ScreenAuth:
		s.WriteString(m.viewAuth())
	case ScreenMain:
		s.WriteString(m.viewMain())
	case ScreenSending:
		s.WriteString(m.viewSending())
	case ScreenResult:
		s.WriteString(m.viewResult())
	}

	return s.String()
}

// viewLoading renders the loading screen
func (m *Model) viewLoading() string {
	return fmt.Sprintf("%s Loading...", m.spinner.View())
}

// viewAuth renders the auth screen
func (m *Model) viewAuth() string {
	var s strings.Builder

	switch m.state.AuthStep {
	case AuthStepPhone:
		s.WriteString(SubtitleStyle.Render("Enter your phone number to sign in"))
		s.WriteString("\n\n")
		s.WriteString(LabelStyle.Render("Phone Number:"))
		s.WriteString("\n")
		s.WriteString(m.phoneInput.View())
	case AuthStepCode:
		s.WriteString(SubtitleStyle.Render("Enter the verification code sent to your phone"))
		s.WriteString("\n\n")
		s.WriteString(LabelStyle.Render("Verification Code:"))
		s.WriteString("\n")
		s.WriteString(m.codeInput.View())
	case AuthStepPassword:
		s.WriteString(SubtitleStyle.Render("Enter your 2FA password"))
		s.WriteString("\n\n")
		s.WriteString(LabelStyle.Render("Password:"))
		s.WriteString("\n")
		s.WriteString(m.passwordInput.View())
	}

	if m.state.Loading {
		s.WriteString("\n\n")
		s.WriteString(m.spinner.View())
		s.WriteString(" Authenticating...")
	}

	if m.state.ErrorMsg != "" {
		s.WriteString("\n\n")
		s.WriteString(ErrorStyle.Render(m.state.ErrorMsg))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("Press Enter to submit"))

	return s.String()
}

// viewMain renders the main screen with folder tabs and chat list
func (m *Model) viewMain() string {
	var s strings.Builder

	// Render folder tabs horizontally
	s.WriteString(m.renderFolderTabs())
	s.WriteString("\n\n")

	// Show search query if active
	if m.state.SearchQuery != "" {
		s.WriteString(SearchStyle.Render(searchText(m.state.SearchQuery)))
		s.WriteString("\n\n")
	}

	// Show loading indicator when switching folders
	if m.state.Loading {
		s.WriteString(fmt.Sprintf("%s Loading chats...", m.spinner.View()))
		s.WriteString("\n")
	} else if len(m.state.Chats) == 0 {
		if m.state.SearchQuery != "" {
			s.WriteString(NormalStyle.Render("No chats match your search"))
		} else {
			s.WriteString(NormalStyle.Render("No chats found in this folder"))
		}
		s.WriteString("\n")
	} else {
		// Render chat list
		maxVisible := m.height - 12
		if maxVisible < 5 {
			maxVisible = 5
		}
		if maxVisible > len(m.state.Chats) {
			maxVisible = len(m.state.Chats)
		}

		start := 0
		if m.state.SelectedChat >= maxVisible {
			start = m.state.SelectedChat - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.state.Chats) {
			end = len(m.state.Chats)
		}

		// Get current folder for pin check
		var currentFolder entity.Folder
		if m.state.SelectedFolder < len(m.state.Folders) {
			currentFolder = m.state.Folders[m.state.SelectedFolder]
		}

		for i := start; i < end; i++ {
			chat := m.state.Chats[i]
			cursor := "  "
			style := NormalStyle
			if i == m.state.SelectedChat {
				cursor = "▸ "
				style = SelectedStyle
			}

			icon := chatTypeIcon(chat.Type)

			// Add pin indicator (check folder-specific pins or global for "All Chats")
			pinIndicator := ""
			if currentFolder.IsAllChats() || currentFolder.IsArchive() {
				if chat.IsPinned {
					pinIndicator = pinText()
				}
			} else {
				if isPinned, _ := currentFolder.IsPinnedInFolder(chat.Peer); isPinned {
					pinIndicator = pinText()
				}
			}

			s.WriteString(cursor)
			s.WriteString(style.Render(fmt.Sprintf("%s%s %s", pinIndicator, icon, chat.DisplayName())))
			s.WriteString("\n")
		}

		if len(m.state.Chats) > maxVisible {
			s.WriteString(fmt.Sprintf("\n%s", HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d chats", start+1, end, len(m.state.Chats)))))
		}
	}

	s.WriteString("\n")
	s.WriteString(HelpStyle.Render("←/→ switch folders • ↑/↓ select chat • Enter to send • Esc to quit"))

	return s.String()
}

// renderFolderTabs renders horizontal folder tabs
func (m *Model) renderFolderTabs() string {
	var tabs []string

	for i, folder := range m.state.Folders {
		name := truncateFolderName(folder.DisplayName(), 15)

		if i == m.state.SelectedFolder {
			tabs = append(tabs, TabActiveStyle.Render(name))
		} else {
			tabs = append(tabs, TabStyle.Render(name))
		}
	}

	return strings.Join(tabs, TabSeparator.String())
}

func truncateFolderName(name string, maxRunes int) string {
	runes := []rune(name)
	if len(runes) <= maxRunes {
		return name
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}

func initialUploadPercent(total int64) float64 {
	if total == 0 {
		return 100
	}
	return 0
}

// viewSending renders the sending screen
func (m *Model) viewSending() string {
	var s strings.Builder

	s.WriteString(fmt.Sprintf("Sending file to %s...\n\n", m.state.SelectedName))

	// Progress bar
	prog := m.state.UploadProgress
	s.WriteString(m.progressBar.ViewAs(prog.Percent / 100))
	s.WriteString("\n\n")

	// Progress info
	uploaded := formatBytes(prog.Uploaded)
	total := formatBytes(prog.Total)
	speed := formatBytes(int64(prog.Speed)) + "/s"

	// Calculate remaining time
	var remaining string
	if prog.Speed > 0 {
		remainingBytes := prog.Total - prog.Uploaded
		remainingSecs := float64(remainingBytes) / prog.Speed
		remaining = formatDuration(remainingSecs)
	} else {
		remaining = "calculating..."
	}

	s.WriteString(fmt.Sprintf("%s / %s  •  %s  •  %s remaining",
		uploaded, total, speed, remaining))

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("Press Esc to cancel"))

	return s.String()
}

// formatBytes formats bytes into human readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// formatDuration formats seconds into human readable duration
func formatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	hours := minutes / 60
	mins := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// updateSending handles sending screen updates
func (m *Model) updateSending(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			// Cancel upload
			if m.sendCancel != nil {
				m.sendCancel()
			}
			return m, nil
		}
	case progress.FrameMsg:
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd
	}
	return m, nil
}

// viewResult renders the result screen
func (m *Model) viewResult() string {
	var s strings.Builder

	if m.state.Success {
		s.WriteString(SuccessStyle.Render("✓ Success!"))
		s.WriteString("\n\n")
		s.WriteString(NormalStyle.Render(m.state.ResultMsg))
	} else {
		s.WriteString(ErrorStyle.Render("✗ Error"))
		s.WriteString("\n\n")
		s.WriteString(NormalStyle.Render(m.state.ErrorMsg))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("Press Enter or Esc to exit"))

	return s.String()
}
