package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"golang.org/x/time/rate"

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
	eventChan   chan AuthEvent
	ready       chan struct{}
	readyOnce   sync.Once
	authFlow    *AuthFlow
	selfMu      sync.RWMutex
	self        *tg.User
}

// AuthEvent represents auth state or error updates for the UI.
type AuthEvent struct {
	State       entity.AuthState
	Err         error
	Recoverable bool
}

// AuthFlow handles authentication flow
type AuthFlow struct {
	phoneChan    chan string
	codeChan     chan string
	passwordChan chan string
}

// NewAuthFlow creates a new auth flow
func NewAuthFlow() *AuthFlow {
	return &AuthFlow{
		phoneChan:    make(chan string, 1),
		codeChan:     make(chan string, 1),
		passwordChan: make(chan string, 1),
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
		eventChan: make(chan AuthEvent, 20),
		ready:     make(chan struct{}),
		authFlow:  NewAuthFlow(),
	}

	return c, nil
}

// Start initializes and starts the client
func (c *Client) Start() error {
	waiter := floodwait.NewWaiter()
	sessionStorage := &session.FileStorage{
		Path: filepath.Join(c.config.SessionDir, "session.json"),
	}

	c.client = telegram.NewClient(c.config.APIID, c.config.APIHash, telegram.Options{
		SessionStorage: sessionStorage,
		Middlewares: []telegram.Middleware{
			waiter,
			ratelimit.New(rate.Every(200*time.Millisecond), 5),
		},
	})

	err := waiter.Run(c.ctx, func(runCtx context.Context) error {
		return c.client.Run(runCtx, func(ctx context.Context) error {
			// Get API client
			c.api = c.client.API()

			statusCtx, cancel := context.WithTimeout(ctx, authRPCTimeout)
			status, err := c.client.Auth().Status(statusCtx)
			cancel()
			if err != nil {
				err = fmt.Errorf("failed to get auth status: %w", err)
				c.publishError(err, false)
				return err
			}

			if status.Authorized {
				if err := c.loadSelf(ctx); err != nil {
					err = fmt.Errorf("failed to load current account: %w", err)
					c.publishError(err, false)
					return err
				}
				c.markReady()
				<-ctx.Done()
				return nil
			}

			c.SetAuthState(entity.AuthStateWaitPhoneNumber)

			for {
				phone, err := c.waitForInput(ctx, c.authFlow.phoneChan)
				if err != nil {
					return nil
				}

				sendCodeCtx, cancel := context.WithTimeout(ctx, authRPCTimeout)
				sentCode, err := c.client.Auth().SendCode(sendCodeCtx, phone, auth.SendCodeOptions{})
				cancel()
				if err != nil {
					if isRecoverablePhoneError(err) {
						c.publishError(err, true)
						c.SetAuthState(entity.AuthStateWaitPhoneNumber)
						continue
					}

					c.publishError(err, false)
					return err
				}

				var phoneCodeHash string
				switch sc := sentCode.(type) {
				case *tg.AuthSentCode:
					phoneCodeHash = sc.PhoneCodeHash
				case *tg.AuthSentCodeSuccess:
					if err := c.loadSelf(ctx); err != nil {
						err = fmt.Errorf("failed to load current account: %w", err)
						c.publishError(err, false)
						return err
					}
					c.markReady()
					<-ctx.Done()
					return nil
				}

				c.SetAuthState(entity.AuthStateWaitCode)

				for {
					code, err := c.waitForInput(ctx, c.authFlow.codeChan)
					if err != nil {
						return nil
					}

					signInCtx, cancel := context.WithTimeout(ctx, authRPCTimeout)
					_, signInErr := c.client.Auth().SignIn(signInCtx, phone, code, phoneCodeHash)
					cancel()
					if signInErr == nil {
						if err := c.loadSelf(ctx); err != nil {
							err = fmt.Errorf("failed to load current account: %w", err)
							c.publishError(err, false)
							return err
						}
						c.markReady()
						<-ctx.Done()
						return nil
					}

					if errors.Is(signInErr, auth.ErrPasswordAuthNeeded) {
						c.SetAuthState(entity.AuthStateWaitPassword)
						for {
							password, err := c.waitForInput(ctx, c.authFlow.passwordChan)
							if err != nil {
								return nil
							}

							passwordCtx, cancel := context.WithTimeout(ctx, authRPCTimeout)
							_, passwordErr := c.client.Auth().Password(passwordCtx, password)
							cancel()
							if passwordErr == nil {
								if err := c.loadSelf(ctx); err != nil {
									err = fmt.Errorf("failed to load current account: %w", err)
									c.publishError(err, false)
									return err
								}
								c.markReady()
								<-ctx.Done()
								return nil
							}

							if isRecoverablePasswordError(passwordErr) {
								c.publishError(passwordErr, true)
								c.SetAuthState(entity.AuthStateWaitPassword)
								continue
							}

							c.publishError(passwordErr, false)
							return passwordErr
						}
					}

					if tgerr.Is(signInErr, "PHONE_CODE_EXPIRED") {
						c.publishError(signInErr, true)
						c.SetAuthState(entity.AuthStateWaitPhoneNumber)
						break
					}

					if isRecoverableCodeError(signInErr) {
						c.publishError(signInErr, true)
						c.SetAuthState(entity.AuthStateWaitCode)
						continue
					}

					c.publishError(signInErr, false)
					return signInErr
				}
			}
		})
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err != nil {
		c.publishError(err, false)
	}
	return err
}

// Close closes the client
func (c *Client) Close() error {
	if c.cancel != nil {
		c.SetAuthState(entity.AuthStateClosed)
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

	c.publishEvent(AuthEvent{State: state})
}

// AuthStateChan returns the auth state change channel
func (c *Client) AuthStateChan() <-chan entity.AuthState {
	return c.authChan
}

// AuthEventChan returns the auth event stream.
func (c *Client) AuthEventChan() <-chan AuthEvent {
	return c.eventChan
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

func (c *Client) markReady() {
	c.SetAuthState(entity.AuthStateReady)
	c.readyOnce.Do(func() {
		close(c.ready)
	})
}

func (c *Client) publishError(err error, recoverable bool) {
	c.publishEvent(AuthEvent{Err: err, Recoverable: recoverable})
}

func (c *Client) publishEvent(event AuthEvent) {
	select {
	case c.eventChan <- event:
	default:
	}
}

func (c *Client) waitForInput(ctx context.Context, ch <-chan string) (string, error) {
	select {
	case value := <-ch:
		return value, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (c *Client) loadSelf(ctx context.Context) error {
	selfCtx, cancel := context.WithTimeout(ctx, profileRPCTimeout)
	defer cancel()

	self, err := c.client.Self(selfCtx)
	if err != nil {
		return err
	}

	c.selfMu.Lock()
	c.self = self
	c.selfMu.Unlock()
	return nil
}

// Self returns the current authorized Telegram user.
func (c *Client) Self(ctx context.Context) (*tg.User, error) {
	c.selfMu.RLock()
	if c.self != nil {
		self := c.self
		c.selfMu.RUnlock()
		return self, nil
	}
	c.selfMu.RUnlock()

	if c.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	if err := c.loadSelf(ctx); err != nil {
		return nil, err
	}

	c.selfMu.RLock()
	defer c.selfMu.RUnlock()
	if c.self == nil {
		return nil, fmt.Errorf("current user is unavailable")
	}
	return c.self, nil
}

// SelfID returns the current authorized Telegram user ID.
func (c *Client) SelfID(ctx context.Context) (int64, error) {
	self, err := c.Self(ctx)
	if err != nil {
		return 0, err
	}
	return self.ID, nil
}

func isRecoverablePhoneError(err error) bool {
	return tgerr.Is(err, "PHONE_NUMBER_INVALID")
}

func isRecoverableCodeError(err error) bool {
	return tgerr.Is(err, "PHONE_CODE_INVALID", "PHONE_CODE_EMPTY")
}

func isRecoverablePasswordError(err error) bool {
	return errors.Is(err, auth.ErrPasswordInvalid) || tgerr.Is(err, "PASSWORD_HASH_INVALID")
}

// SendPhoneNumber sends the phone number
func (af *AuthFlow) SendPhoneNumber(phone string) {
	sendLatest(af.phoneChan, phone)
}

// SendCode sends the verification code
func (af *AuthFlow) SendCode(code string) {
	sendLatest(af.codeChan, code)
}

// SendPassword sends the 2FA password
func (af *AuthFlow) SendPassword(password string) {
	sendLatest(af.passwordChan, password)
}

func sendLatest(ch chan string, value string) {
	select {
	case ch <- value:
		return
	default:
	}

	select {
	case <-ch:
	default:
	}

	ch <- value
}
