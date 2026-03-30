# SendTG

A TUI application to send files to Telegram contacts directly from the command line.

## Features

- 📤 Send files to any Telegram contact
- 📁 Browse chat folders
- 🔐 Full authentication support (including 2FA)
- 🎨 Beautiful terminal user interface
- 🚀 Pure Go implementation (no external dependencies like TDLib)
- 🔒 API credentials embedded at build time

## Prerequisites

### Go

Go 1.21 or later is required.

## Building

The application requires Telegram API credentials to be embedded at build time.

1. Get your API credentials from https://my.telegram.org:
   - Go to "API development tools"
   - Create a new application if you don't have one
   - Note down your `api_id` and `api_hash`

2. Create `.env` file from the example:

```bash
cp .env.example .env
```

3. Edit `.env` with your credentials:

```bash
API_ID=123456
API_HASH=your_api_hash_here
```

4. Build using Make:

```bash
make build
```

Or build manually:

```bash
source .env
go build -ldflags "-s -w -X 'sendtg/internal/config.apiID=${API_ID}' -X 'sendtg/internal/config.apiHash=${API_HASH}'" -o sendtg cmd/main.go
```

## Usage

```bash
sendtg <filename>
```

For example:

```bash
sendtg document.pdf
sendtg ~/Downloads/photo.jpg
```

### Controls

- **↑/↓** - Navigate through chats
- **←/→** or **Tab/Shift+Tab** - Switch folders
- **Enter** - Send the file to the selected chat
- **Esc** - Clear search, cancel upload, or exit the application

## Project Structure

```
sendtg/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration handling
│   │   └── config.go
│   ├── domain/                 # Domain layer (entities & interfaces)
│   │   ├── entity/
│   │   │   ├── auth_state.go
│   │   │   ├── chat.go
│   │   │   ├── contact.go
│   │   │   ├── file.go
│   │   │   └── folder.go
│   │   └── repository/
│   │       ├── auth_repository.go
│   │       ├── chat_repository.go
│   │       └── file_repository.go
│   ├── infrastructure/         # External dependencies
│   │   └── telegram/
│   │       ├── auth_repo.go
│   │       ├── authorizer.go
│   │       ├── chat_repo.go
│   │       ├── client.go
│   │       └── file_repo.go
│   ├── ui/                     # TUI layer (bubbletea)
│   │   ├── app.go
│   │   ├── model.go
│   │   ├── state.go
│   │   └── styles.go
│   └── usecase/                # Application logic
│       ├── auth/
│       │   └── auth_usecase.go
│       ├── chat/
│       │   └── chat_usecase.go
│       └── file/
│           └── file_usecase.go
├── go.mod
├── go.sum
└── README.md
```

## Architecture

This project follows Clean Architecture principles:

1. **Domain Layer** (`internal/domain/`) - Contains business entities and repository interfaces
2. **Use Case Layer** (`internal/usecase/`) - Contains application-specific business logic
3. **Infrastructure Layer** (`internal/infrastructure/`) - Contains implementations for external services (Telegram)
4. **Presentation Layer** (`internal/ui/`) - Contains the TUI implementation

## License

MIT License
