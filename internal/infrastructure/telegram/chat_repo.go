package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
	"sendtg/internal/infrastructure/cache"
)

// ChatRepository implements the ChatRepository interface
type ChatRepository struct {
	client *Client
	cache  *cache.DialogCache
}

// NewChatRepository creates a new chat repository
func NewChatRepository(c *Client, dialogCache *cache.DialogCache) *ChatRepository {
	return &ChatRepository{
		client: c,
		cache:  dialogCache,
	}
}

// GetFolders returns the list of chat folders
func (r *ChatRepository) GetFolders() ([]entity.Folder, error) {
	api := r.client.GetAPI()
	if api == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()

	// Get chat folders
	result, err := api.MessagesGetDialogFilters(ctx)
	if err != nil {
		// If error, just return "All Chats" folder
		return []entity.Folder{
			{ID: 0, Title: "All Chats"},
		}, nil
	}

	folders := make([]entity.Folder, 0, len(result.Filters)+1)

	// Add "All Chats" as the first folder
	folders = append(folders, entity.Folder{
		ID:    0,
		Title: "All Chats",
	})

	// Add user folders
	for _, filter := range result.Filters {
		switch f := filter.(type) {
		case *tg.DialogFilterDefault:
			// This is the default "All Chats" folder, skip it as we already added it
			continue
		case *tg.DialogFilter:
			folder := entity.Folder{
				ID:                 int32(f.ID),
				Title:              ExtractTextWithCustomEmoji(f.Title),
				IncludeContacts:    f.Contacts,
				IncludeNonContacts: f.NonContacts,
				IncludeGroups:      f.Groups,
				IncludeBroadcasts:  f.Broadcasts,
				IncludeBots:        f.Bots,
				IncludedPeerIDs:    extractPeerIDs(f.IncludePeers),
				ExcludedPeerIDs:    extractPeerIDs(f.ExcludePeers),
				PinnedPeerIDs:      extractPeerIDs(f.PinnedPeers),
			}
			folders = append(folders, folder)
		case *tg.DialogFilterChatlist:
			folder := entity.Folder{
				ID:              int32(f.ID),
				Title:           ExtractTextWithCustomEmoji(f.Title),
				IncludedPeerIDs: extractPeerIDs(f.IncludePeers),
				PinnedPeerIDs:   extractPeerIDs(f.PinnedPeers),
			}
			folders = append(folders, folder)
		}
	}

	return folders, nil
}

// extractPeerIDs extracts peer IDs from InputPeer slice
func extractPeerIDs(peers []tg.InputPeerClass) []int64 {
	ids := make([]int64, 0, len(peers))
	for _, peer := range peers {
		switch p := peer.(type) {
		case *tg.InputPeerUser:
			ids = append(ids, p.UserID)
		case *tg.InputPeerChat:
			ids = append(ids, p.ChatID)
		case *tg.InputPeerChannel:
			ids = append(ids, p.ChannelID)
		case *tg.InputPeerUserFromMessage:
			ids = append(ids, p.UserID)
		case *tg.InputPeerChannelFromMessage:
			ids = append(ids, p.ChannelID)
		}
	}
	return ids
}

// GetChatsByFolder returns chats in a specific folder
func (r *ChatRepository) GetChatsByFolder(folderID int32) ([]entity.Chat, error) {
	api := r.client.GetAPI()
	if api == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()

	// Get dialogs (chats)
	dialogs, err := r.getDialogs(ctx, api, folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dialogs: %w", err)
	}

	return dialogs, nil
}

// GetAllChats returns all chats (loads from server and updates cache)
func (r *ChatRepository) GetAllChats() ([]entity.Chat, error) {
	chats, err := r.GetChatsByFolder(0)
	if err != nil {
		return nil, err
	}

	// Update cache in background
	if r.cache != nil {
		go r.cache.SetChats(chats)
	}

	return chats, nil
}

// GetCachedChats returns chats from cache (instant)
func (r *ChatRepository) GetCachedChats() []entity.Chat {
	if r.cache == nil {
		return nil
	}
	return r.cache.GetChats()
}

// GetCachedFolders returns folders from cache (instant)
func (r *ChatRepository) GetCachedFolders() []entity.Folder {
	if r.cache == nil {
		return nil
	}
	return r.cache.GetFolders()
}

// HasCachedData returns true if there's cached data available
func (r *ChatRepository) HasCachedData() bool {
	if r.cache == nil {
		return false
	}
	return r.cache.HasData()
}

// GetChatsFirstPage returns first page of chats quickly (for fast startup)
func (r *ChatRepository) GetChatsFirstPage(limit int) ([]entity.Chat, error) {
	api := r.client.GetAPI()
	if api == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()

	// Get blocked users first
	blockedUsers := r.getBlockedUserIDs(ctx, api)

	req := &tg.MessagesGetDialogsRequest{
		OffsetDate: 0,
		OffsetID:   0,
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      limit,
		Hash:       0,
	}

	result, err := api.MessagesGetDialogs(ctx, req)
	if err != nil {
		return nil, err
	}

	var dialogs []tg.DialogClass
	var messages []tg.MessageClass
	var tgChats []tg.ChatClass
	var users []tg.UserClass

	switch res := result.(type) {
	case *tg.MessagesDialogs:
		dialogs = res.Dialogs
		messages = res.Messages
		tgChats = res.Chats
		users = res.Users
	case *tg.MessagesDialogsSlice:
		dialogs = res.Dialogs
		messages = res.Messages
		tgChats = res.Chats
		users = res.Users
	case *tg.MessagesDialogsNotModified:
		return nil, nil
	}

	return r.processDialogs(dialogs, messages, tgChats, users, blockedUsers)
}

// UpdateFoldersCache updates the folders cache
func (r *ChatRepository) UpdateFoldersCache(folders []entity.Folder) {
	if r.cache != nil {
		go r.cache.SetFolders(folders)
	}
}

// getBlockedUserIDs returns a set of blocked user IDs
func (r *ChatRepository) getBlockedUserIDs(ctx context.Context, api *tg.Client) map[int64]bool {
	blocked := make(map[int64]bool)

	result, err := api.ContactsGetBlocked(ctx, &tg.ContactsGetBlockedRequest{
		Limit: 1000, // Get up to 1000 blocked users
	})
	if err != nil {
		return blocked
	}

	switch res := result.(type) {
	case *tg.ContactsBlocked:
		for _, peer := range res.Blocked {
			if p, ok := peer.PeerID.(*tg.PeerUser); ok {
				blocked[p.UserID] = true
			}
		}
	case *tg.ContactsBlockedSlice:
		for _, peer := range res.Blocked {
			if p, ok := peer.PeerID.(*tg.PeerUser); ok {
				blocked[p.UserID] = true
			}
		}
	}

	return blocked
}

// getDialogs fetches all dialogs from Telegram with pagination
func (r *ChatRepository) getDialogs(ctx context.Context, api *tg.Client, folderID int32) ([]entity.Chat, error) {
	// Get blocked users first
	blockedUsers := r.getBlockedUserIDs(ctx, api)

	var allDialogs []tg.DialogClass
	var allMessages []tg.MessageClass
	var allTgChats []tg.ChatClass
	var allUsers []tg.UserClass

	// Pagination variables
	offsetDate := 0
	offsetID := 0
	var offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}
	limit := 100

	for {
		req := &tg.MessagesGetDialogsRequest{
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			OffsetPeer: offsetPeer,
			Limit:      limit,
			Hash:       0,
		}

		// FolderID in MessagesGetDialogs only works for archive (1)
		if folderID == 1 {
			req.FolderID = 1
		}

		result, err := api.MessagesGetDialogs(ctx, req)
		if err != nil {
			return nil, err
		}

		var dialogs []tg.DialogClass
		var messages []tg.MessageClass
		var tgChats []tg.ChatClass
		var users []tg.UserClass
		var totalCount int

		switch res := result.(type) {
		case *tg.MessagesDialogs:
			// All dialogs returned at once
			dialogs = res.Dialogs
			messages = res.Messages
			tgChats = res.Chats
			users = res.Users
			totalCount = len(dialogs)
		case *tg.MessagesDialogsSlice:
			dialogs = res.Dialogs
			messages = res.Messages
			tgChats = res.Chats
			users = res.Users
			totalCount = res.Count
		case *tg.MessagesDialogsNotModified:
			// No changes
			break
		}

		// Append to accumulated results
		allDialogs = append(allDialogs, dialogs...)
		allMessages = append(allMessages, messages...)
		allTgChats = append(allTgChats, tgChats...)
		allUsers = append(allUsers, users...)

		// Check if we got all dialogs
		if len(dialogs) < limit || len(allDialogs) >= totalCount {
			break
		}

		// Update offset for next page
		if len(dialogs) > 0 {
			lastDialog := dialogs[len(dialogs)-1]
			if d, ok := lastDialog.(*tg.Dialog); ok {
				// Find the last message date for offset
				for _, m := range messages {
					if msg, ok := m.(*tg.Message); ok {
						if getPeerID(msg.PeerID) == getPeerID(d.Peer) {
							offsetDate = msg.Date
							offsetID = msg.ID
							break
						}
					}
				}
				// Create offset peer
				offsetPeer = peerToInputPeer(d.Peer, allUsers, allTgChats)
			}
		}
	}

	// Process accumulated results
	return r.processDialogs(allDialogs, allMessages, allTgChats, allUsers, blockedUsers)
}

// peerToInputPeer converts PeerClass to InputPeerClass
func peerToInputPeer(peer tg.PeerClass, users []tg.UserClass, chats []tg.ChatClass) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerUser:
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return &tg.InputPeerUser{
					UserID:     user.ID,
					AccessHash: user.AccessHash,
				}
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		for _, c := range chats {
			if channel, ok := c.(*tg.Channel); ok && channel.ID == p.ChannelID {
				return &tg.InputPeerChannel{
					ChannelID:  channel.ID,
					AccessHash: channel.AccessHash,
				}
			}
		}
	}
	return &tg.InputPeerEmpty{}
}

// processDialogs processes dialogs and extracts chat entities
func (r *ChatRepository) processDialogs(dialogs []tg.DialogClass, messages []tg.MessageClass, tgChats []tg.ChatClass, users []tg.UserClass, blockedUsers map[int64]bool) ([]entity.Chat, error) {
	// Build message date map (peer -> last message date)
	msgDateMap := make(map[int64]int)
	for _, m := range messages {
		if msg, ok := m.(*tg.Message); ok {
			peerID := getPeerID(msg.PeerID)
			if peerID != 0 {
				if existing, ok := msgDateMap[peerID]; !ok || msg.Date > existing {
					msgDateMap[peerID] = msg.Date
				}
			}
		}
	}

	// Build user map
	userMap := make(map[int64]*tg.User)
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}

	// Build chat map
	chatMap := make(map[int64]tg.ChatClass)
	for _, c := range tgChats {
		switch chat := c.(type) {
		case *tg.Chat:
			chatMap[chat.ID] = c
		case *tg.Channel:
			chatMap[chat.ID] = c
		}
	}

	// Extract chats from dialogs in order (with deduplication)
	chats := make([]entity.Chat, 0, len(dialogs))
	seenKeys := make(map[string]bool)
	pinOrder := 0
	for _, d := range dialogs {
		dialog, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}

		// Create unique key based on peer type and ID
		peerKey := getPeerKey(dialog.Peer)

		// Skip duplicates
		if seenKeys[peerKey] {
			continue
		}
		seenKeys[peerKey] = true

		peerID := getPeerID(dialog.Peer)
		lastMsgDate := msgDateMap[peerID]
		isPinned := dialog.Pinned

		// Track pin order for pinned chats
		currentPinOrder := -1
		if isPinned {
			currentPinOrder = pinOrder
			pinOrder++
		}

		// Current position in the dialog list (used for sorting)
		currentOrder := len(chats)

		switch peer := dialog.Peer.(type) {
		case *tg.PeerUser:
			user, ok := userMap[peer.UserID]
			if !ok || user.Deleted {
				continue
			}

			// Check if user is blocked
			canWrite := !blockedUsers[user.ID]

			name := user.FirstName
			if user.LastName != "" {
				name += " " + user.LastName
			}
			chats = append(chats, entity.Chat{
				ID:              user.ID,
				Title:           name,
				Type:            entity.ChatTypePrivate,
				Username:        user.Username,
				LastMessageDate: lastMsgDate,
				IsContact:       user.Contact,
				IsBot:           user.Bot,
				IsPinned:        isPinned,
				PinOrder:        currentPinOrder,
				Order:           currentOrder,
				CanWrite:        canWrite,
			})

		case *tg.PeerChat:
			c, ok := chatMap[peer.ChatID]
			if !ok {
				continue
			}
			if chat, ok := c.(*tg.Chat); ok {
				// Skip migrated groups - their supergroups are already in the list
				// at the correct position based on last message date
				if chat.MigratedTo != nil {
					continue
				}

				// Check if user can write to this group
				canWrite := !chat.Left && !chat.Deactivated
				if canWrite && chat.DefaultBannedRights.SendMessages {
					canWrite = false
				}

				chats = append(chats, entity.Chat{
					ID:              chat.ID,
					Title:           chat.Title,
					Type:            entity.ChatTypeGroup,
					LastMessageDate: lastMsgDate,
					IsPinned:        isPinned,
					PinOrder:        currentPinOrder,
					Order:           currentOrder,
					CanWrite:        canWrite,
				})
			}

		case *tg.PeerChannel:
			c, ok := chatMap[peer.ChannelID]
			if !ok {
				continue
			}
			if channel, ok := c.(*tg.Channel); ok {
				chatType := entity.ChatTypeSupergroup
				if channel.Broadcast {
					chatType = entity.ChatTypeChannel
				}

				// Check if user can write to this channel/supergroup
				canWrite := !channel.Left

				if channel.Broadcast {
					// For broadcast channels: can write only if creator or has PostMessages admin right
					canWrite = false
					if channel.Creator {
						canWrite = true
					} else if adminRights, ok := channel.GetAdminRights(); ok {
						canWrite = adminRights.PostMessages
					}
				} else {
					// For supergroups: check DefaultBannedRights
					if canWrite {
						if rights, ok := channel.GetDefaultBannedRights(); ok && rights.SendMessages {
							canWrite = false
						}
					}
				}

				chats = append(chats, entity.Chat{
					ID:              channel.ID,
					Title:           channel.Title,
					Type:            chatType,
					Username:        channel.Username,
					LastMessageDate: lastMsgDate,
					IsPinned:        isPinned,
					PinOrder:        currentPinOrder,
					Order:           currentOrder,
					CanWrite:        canWrite,
				})
			}
		}
	}

	return chats, nil
}

// getPeerKey returns a unique string key for a peer (type:id)
func getPeerKey(peer tg.PeerClass) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("user:%d", p.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("chat:%d", p.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("channel:%d", p.ChannelID)
	}
	return ""
}

// getPeerID extracts peer ID from PeerClass
func getPeerID(peer tg.PeerClass) int64 {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return p.ChatID
	case *tg.PeerChannel:
		return p.ChannelID
	}
	return 0
}
