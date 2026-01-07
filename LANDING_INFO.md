# SendTG - Landing Page Information

## Project Overview

**SendTG** is a command-line TUI (Terminal User Interface) application for quickly sending files to Telegram contacts directly from the terminal.

## Tagline Options

- "Send files to Telegram from your terminal in seconds"
- "The fastest way to share files via Telegram"
- "Terminal-native Telegram file sharing"

## Key Features

### 🚀 Quick File Sharing
Send any file to your Telegram contacts without opening the Telegram app. Just run `sendtg filename.txt` and select the recipient.

### 📁 Folder Support
Navigate through your Telegram folders exactly like in the official app. All your custom folders are displayed as tabs for easy navigation.

### 🔍 Instant Search
Start typing to filter chats instantly. No need to press any buttons - search begins as you type. Supports Latin and Cyrillic characters.

### 📌 Pinned Chats
Pinned chats are displayed at the top, respecting both global pins and folder-specific pins, just like in Telegram.

### 🔐 Full Authentication Support
Complete Telegram authentication including:
- Phone number verification
- SMS/Telegram code input
- Two-factor authentication (2FA) password support

### 📊 Upload Progress
Real-time upload progress bar showing:
- Bytes uploaded / Total size
- Upload speed
- Estimated time remaining
- Ability to cancel upload with Esc

### 💾 Smart Caching
- Dialogs are cached locally for instant startup
- Background refresh keeps data up-to-date
- Cross-platform cache storage

### 🖥️ Cross-Platform
Works on:
- **Linux** - Native support
- **macOS** - Full compatibility
- **Windows** - Full compatibility

## Technical Details

### Built With
- **Language**: Go (Golang)
- **TUI Framework**: Bubbletea (Charm)
- **Telegram API**: gotd/td (MTProto)
- **Architecture**: Clean Architecture

### Installation

```bash
# Clone the repository
git clone https://github.com/username/sendtg.git

# Build
cd sendtg
make build

# Run
./sendtg path/to/file.txt
```

### Usage

```bash
# Send a file
sendtg document.pdf

# Send an image
sendtg photo.jpg

# Send any file type
sendtg archive.zip
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←` / `→` | Switch between folders |
| `↑` / `↓` | Navigate chat list |
| `Enter` | Select chat and send file |
| `Esc` | Clear search / Cancel / Quit |
| `Backspace` | Delete search character |
| Type any character | Start searching |

## Screenshots Description

### Main Screen
- Header with app title and file info (name + size)
- Horizontal folder tabs at the top
- Scrollable chat list with icons:
  - 👤 Private chats
  - 👥 Groups and supergroups
  - 📢 Channels
  - 📌 Pinned indicator
- Search bar appears when typing
- Help text at the bottom

### Authentication Screen
- Clean input fields for:
  - Phone number
  - Verification code
  - 2FA password (if enabled)
- Loading spinner during verification

### Upload Screen
- Progress bar with percentage
- Upload statistics (speed, ETA)
- Cancel option

### Success Screen
- Confirmation message with recipient name
- File name confirmation

## Target Audience

1. **Developers** - Who live in the terminal and want quick file sharing
2. **System Administrators** - For sharing logs, configs, scripts
3. **Power Users** - Who prefer keyboard-driven workflows
4. **DevOps Engineers** - For quick sharing from remote servers

## Use Cases

1. **Share code snippets** - Quickly send code files to colleagues
2. **Send logs** - Share log files for debugging
3. **Transfer configs** - Send configuration files
4. **Quick screenshots** - Send images without leaving terminal
5. **Server file sharing** - Send files from SSH sessions
6. **Automation** - Integrate into scripts and workflows

## Unique Selling Points

1. **No context switching** - Stay in your terminal workflow
2. **Instant startup** - Cached data means no waiting
3. **Keyboard-first** - No mouse needed
4. **Lightweight** - Single binary, no dependencies
5. **Secure** - Your credentials stay local, direct MTProto connection
6. **Open Source** - Transparent and auditable

## Security Features

- Direct connection to Telegram servers via MTProto
- Session stored locally in OS-specific secure location
- No third-party servers involved
- API credentials embedded at build time

## Data Storage Locations

| OS | Configuration | Cache |
|----|---------------|-------|
| Linux | `~/.config/sendtg/` | `~/.cache/sendtg/` |
| macOS | `~/Library/Application Support/sendtg/` | `~/Library/Caches/sendtg/` |
| Windows | `%APPDATA%\sendtg\` | `%LOCALAPPDATA%\sendtg\` |

## Author

**Yury Ermoshin**
- Telegram: [@INDIW33D](https://t.me/INDIW33D)

## License

MIT License (or specify your license)

## Links

- GitHub Repository: [github.com/username/sendtg](https://github.com/username/sendtg)
- Telegram Contact: [@INDIW33D](https://t.me/INDIW33D)

---

## Design Suggestions for Landing Page

### Color Scheme
- Primary: Purple (#7C3AED) - matches the TUI theme
- Secondary: Green (#10B981) - for success states
- Background: Dark (#1a1a2e or similar) - terminal aesthetic
- Text: Light gray (#F3F4F6)

### Visual Elements
- Terminal mockup showing the app in action
- Animated GIF/video of the workflow
- Icons for features (emoji-style to match the app)
- Code blocks for installation commands

### Sections Suggested
1. Hero - Tagline + Terminal demo
2. Features - Grid of key features
3. How it Works - 3-step process
4. Installation - Code blocks
5. Screenshots/Demo - Visual showcase
6. Use Cases - Who benefits
7. Download/CTA - Get started button
8. Footer - Links, author, license

### Tone
- Professional but friendly
- Developer-focused
- Concise and practical
- Emphasize speed and efficiency

