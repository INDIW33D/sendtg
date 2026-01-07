package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
)

// Config holds the configuration for the Telegram client
type Config struct {
	APIID          int
	APIHash        string
	SessionDir     string
	SystemLanguage string
	DeviceModel    string
	AppVersion     string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	// Use cross-platform config directory
	// Linux: ~/.config/sendtg
	// macOS: ~/Library/Application Support/sendtg
	// Windows: %APPDATA%\sendtg
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".sendtg")
	} else {
		configDir = filepath.Join(configDir, "sendtg")
	}

	return &Config{
		APIID:          0,  // Must be set by user
		APIHash:        "", // Must be set by user
		SessionDir:     filepath.Join(configDir, "session"),
		SystemLanguage: "en",
		DeviceModel:    "Desktop",
		AppVersion:     "1.0.0",
	}
}

// Client wraps the Telegram client
type Client struct {
	client      *telegram.Client
	api         *tg.Client
	config      *Config
	ctx         context.Context
	cancel      context.CancelFunc
	authState   entity.AuthState
	authStateMu sync.RWMutex
	authChan    chan entity.AuthState
	ready       chan struct{}
	authFlow    *AuthFlow
}

// AuthFlow handles authentication flow
type AuthFlow struct {
	phoneChan    chan string
	codeChan     chan string
	passwordChan chan string
	errChan      chan error
}

// NewAuthFlow creates a new auth flow
func NewAuthFlow() *AuthFlow {
	return &AuthFlow{
		phoneChan:    make(chan string, 1),
		codeChan:     make(chan string, 1),
		passwordChan: make(chan string, 1),
		errChan:      make(chan error, 1),
	}
}

// NewClientWithManualAuth creates a new Telegram client with manual auth
func NewClientWithManualAuth(config *Config) (*Client, error) {
	// Ensure session directory exists
	if err := os.MkdirAll(config.SessionDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		authState: entity.AuthStateUnknown,
		authChan:  make(chan entity.AuthState, 10),
		ready:     make(chan struct{}),
		authFlow:  NewAuthFlow(),
	}

	return c, nil
}

// Start initializes and starts the client
func (c *Client) Start() error {
	sessionStorage := &session.FileStorage{
		Path: filepath.Join(c.config.SessionDir, "session.json"),
	}

	c.client = telegram.NewClient(c.config.APIID, c.config.APIHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	return c.client.Run(c.ctx, func(ctx context.Context) error {
		// Get API client
		c.api = c.client.API()

		// Check auth status
		status, err := c.client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to get auth status: %w", err)
		}

		if status.Authorized {
			c.SetAuthState(entity.AuthStateReady)
			close(c.ready)
			// Keep connection alive
			<-ctx.Done()
			return nil
		}

		// Need to authenticate
		c.SetAuthState(entity.AuthStateWaitPhoneNumber)

		// Wait for phone number
		phone := <-c.authFlow.phoneChan

		// Send code
		sentCode, err := c.client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		if err != nil {
			c.authFlow.errChan <- err
			return err
		}

		// Extract phone code hash
		var phoneCodeHash string
		switch sc := sentCode.(type) {
		case *tg.AuthSentCode:
			phoneCodeHash = sc.PhoneCodeHash
		case *tg.AuthSentCodeSuccess:
			// Already authenticated
			c.SetAuthState(entity.AuthStateReady)
			close(c.ready)
			<-ctx.Done()
			return nil
		}

		c.SetAuthState(entity.AuthStateWaitCode)

		// Wait for code
		code := <-c.authFlow.codeChan

		// Try to sign in
		_, signInErr := c.client.Auth().SignIn(ctx, phone, code, phoneCodeHash)
		if signInErr != nil {
			// Check if 2FA is required (SESSION_PASSWORD_NEEDED error)
			if errors.Is(signInErr, auth.ErrPasswordAuthNeeded) {
				// 2FA is required
				c.SetAuthState(entity.AuthStateWaitPassword)
				password := <-c.authFlow.passwordChan

				_, err := c.client.Auth().Password(ctx, password)
				if err != nil {
					c.authFlow.errChan <- err
					return err
				}
			} else {
				c.authFlow.errChan <- signInErr
				return signInErr
			}
		}

		c.SetAuthState(entity.AuthStateReady)
		close(c.ready)

		// Keep connection alive
		<-ctx.Done()
		return nil
	})
}

// Close closes the client
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

// GetAuthState returns the current auth state
func (c *Client) GetAuthState() (entity.AuthState, error) {
	c.authStateMu.RLock()
	defer c.authStateMu.RUnlock()
	return c.authState, nil
}

// SetAuthState sets the current auth state
func (c *Client) SetAuthState(state entity.AuthState) {
	c.authStateMu.Lock()
	c.authState = state
	c.authStateMu.Unlock()

	// Non-blocking send to channel
	select {
	case c.authChan <- state:
	default:
	}
}

// AuthStateChan returns the auth state change channel
func (c *Client) AuthStateChan() <-chan entity.AuthState {
	return c.authChan
}

// WaitReady waits for the client to be ready
func (c *Client) WaitReady() {
	<-c.ready
}

// GetAPI returns the Telegram API client
func (c *Client) GetAPI() *tg.Client {
	return c.api
}

// GetContext returns the client context
func (c *Client) GetContext() context.Context {
	return c.ctx
}

// GetAuthFlow returns the auth flow
func (c *Client) GetAuthFlow() *AuthFlow {
	return c.authFlow
}

// SendPhoneNumber sends the phone number
func (af *AuthFlow) SendPhoneNumber(phone string) {
	af.phoneChan <- phone
}

// SendCode sends the verification code
func (af *AuthFlow) SendCode(code string) {
	af.codeChan <- code
}

// SendPassword sends the 2FA password
func (af *AuthFlow) SendPassword(password string) {
	af.passwordChan <- password
}

// ErrChan returns the error channel
func (af *AuthFlow) ErrChan() <-chan error {
	return af.errChan
}
