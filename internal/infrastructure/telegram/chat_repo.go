package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotd/td/tg"

	"sendtg/internal/domain/entity"
	"sendtg/internal/infrastructure/cache"
)

// ChatRepository implements the ChatRepository interface
type ChatRepository struct {
	client         *Client
	cache          *cache.DialogCache
	blockedUsersMu sync.RWMutex
	blockedUsers   map[int64]bool
	blockedUsersAt time.Time
}

const blockedUsersCacheTTL = 5 * time.Minute

const (
	mainPeerFolderID    int32 = 0
	archivePeerFolderID int32 = 1
)

// NewChatRepository creates a new chat repository
func NewChatRepository(c *Client, dialogCache *cache.DialogCache) *ChatRepository {
	return &ChatRepository{
		client:       c,
		cache:        dialogCache,
		blockedUsers: make(map[int64]bool),
	}
}

// GetFolders returns the list of chat folders
func (r *ChatRepository) GetFolders() ([]entity.Folder, error) {
	api := r.client.GetAPI()
	if api == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()
	selfID, _ := r.currentAccountID()

	foldersCtx, cancel := context.WithTimeout(ctx, dialogsRPCTimeout)
	result, err := api.MessagesGetDialogFilters(foldersCtx)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to get dialog filters: %w", err)
	}

	folders := make([]entity.Folder, 0, len(result.Filters)+2)
	allChatsAdded := false

	appendAllChats := func() {
		if allChatsAdded {
			return
		}
		folders = append(folders, entity.Folder{
			Kind:  entity.FolderKindAll,
			Title: "All Chats",
		})
		allChatsAdded = true
	}

	for _, filter := range result.Filters {
		switch f := filter.(type) {
		case *tg.DialogFilterDefault:
			appendAllChats()
		case *tg.DialogFilter:
			folder := entity.Folder{
				Kind:               entity.FolderKindCustom,
				ID:                 int32(f.ID),
				Title:              ExtractTextWithCustomEmoji(f.Title),
				IncludeContacts:    f.Contacts,
				IncludeNonContacts: f.NonContacts,
				IncludeGroups:      f.Groups,
				IncludeBroadcasts:  f.Broadcasts,
				IncludeBots:        f.Bots,
				ExcludeMuted:       f.ExcludeMuted,
				ExcludeRead:        f.ExcludeRead,
				ExcludeArchived:    f.ExcludeArchived,
				IncludedPeers:      extractPeers(f.IncludePeers, selfID),
				ExcludedPeers:      extractPeers(f.ExcludePeers, selfID),
				PinnedPeers:        extractPeers(f.PinnedPeers, selfID),
			}
			folders = append(folders, folder)
		case *tg.DialogFilterChatlist:
			folder := entity.Folder{
				Kind:          entity.FolderKindCustom,
				ID:            int32(f.ID),
				Title:         ExtractTextWithCustomEmoji(f.Title),
				IncludedPeers: extractPeers(f.IncludePeers, selfID),
				PinnedPeers:   extractPeers(f.PinnedPeers, selfID),
			}
			folders = append(folders, folder)
		}
	}

	if !allChatsAdded {
		folders = append([]entity.Folder{{Kind: entity.FolderKindAll, Title: "All Chats"}}, folders...)
	}

	folders = append(folders, entity.Folder{Kind: entity.FolderKindArchive, Title: "Archive"})

	return folders, nil
}

// GetChatsByFolder returns chats from a peer folder (0 = main, 1 = archive).
func (r *ChatRepository) GetChatsByFolder(folderID int32) ([]entity.Chat, error) {
	api := r.client.GetAPI()
	if api == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()
	blockedUsers := r.getBlockedUserIDs(ctx, api)

	dialogs, err := r.getDialogsForPeerFolder(ctx, api, folderID, blockedUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get dialogs: %w", err)
	}

	return dialogs, nil
}

// GetAllChats returns the full dialog inventory from main dialogs and archive.
func (r *ChatRepository) GetAllChats() ([]entity.Chat, error) {
	api := r.client.GetAPI()
	if api == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx := r.client.GetContext()
	blockedUsers := r.getBlockedUserIDs(ctx, api)

	mainChats, err := r.getDialogsForPeerFolder(ctx, api, mainPeerFolderID, blockedUsers)
	if err != nil {
		return nil, err
	}
	archiveChats, err := r.getDialogsForPeerFolder(ctx, api, archivePeerFolderID, blockedUsers)
	if err != nil {
		return nil, err
	}

	chats := mergeChatInventories(mainChats, archiveChats)

	// Update cache in background
	if r.cache != nil {
		if accountID, err := r.currentAccountID(); err == nil {
			go r.cache.SetChats(accountID, chats)
		}
	}

	return chats, nil
}

// GetCachedChats returns chats from cache (instant)
func (r *ChatRepository) GetCachedChats() []entity.Chat {
	if r.cache == nil {
		return nil
	}
	if !r.canUseCache() {
		return nil
	}
	return r.cache.GetChats()
}

// GetCachedFolders returns folders from cache (instant)
func (r *ChatRepository) GetCachedFolders() []entity.Folder {
	if r.cache == nil {
		return nil
	}
	if !r.canUseCache() {
		return nil
	}
	return r.cache.GetFolders()
}

// HasCachedData returns true if there's cached data available
func (r *ChatRepository) HasCachedData() bool {
	if r.cache == nil {
		return false
	}
	return r.canUseCache() && r.cache.HasData()
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

	pageCtx, cancel := context.WithTimeout(ctx, dialogsRPCTimeout)
	result, err := api.MessagesGetDialogs(pageCtx, req)
	cancel()
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

	return r.processDialogs(dialogs, messages, tgChats, users, blockedUsers, mainPeerFolderID)
}

// UpdateFoldersCache updates the folders cache
func (r *ChatRepository) UpdateFoldersCache(folders []entity.Folder) {
	if r.cache != nil {
		if accountID, err := r.currentAccountID(); err == nil {
			go r.cache.SetFolders(accountID, folders)
		}
	}
}

func (r *ChatRepository) currentAccountID() (int64, error) {
	ctx, cancel := context.WithTimeout(r.client.GetContext(), profileRPCTimeout)
	defer cancel()
	return r.client.SelfID(ctx)
}

func (r *ChatRepository) canUseCache() bool {
	if r.cache == nil || !r.cache.IsValid(cacheMaxAge) {
		return false
	}
	accountID, err := r.currentAccountID()
	if err != nil {
		return false
	}
	return r.cache.GetAccountID() == accountID
}

// getBlockedUserIDs returns a set of blocked user IDs
func (r *ChatRepository) getBlockedUserIDs(ctx context.Context, api *tg.Client) map[int64]bool {
	if blocked, ok := r.getCachedBlockedUsers(true); ok {
		return blocked
	}

	blocked := make(map[int64]bool)
	offset := 0
	limit := 100

	for {
		blockedCtx, cancel := context.WithTimeout(ctx, blockedUsersRPCTimeout)
		result, err := api.ContactsGetBlocked(blockedCtx, &tg.ContactsGetBlockedRequest{
			Offset: offset,
			Limit:  limit,
		})
		cancel()
		if err != nil {
			if cached, ok := r.getCachedBlockedUsers(false); ok {
				return cached
			}
			return blocked
		}

		pageCount := 0
		switch res := result.(type) {
		case *tg.ContactsBlocked:
			pageCount = len(res.Blocked)
			for _, peer := range res.Blocked {
				if p, ok := peer.PeerID.(*tg.PeerUser); ok {
					blocked[p.UserID] = true
				}
			}
		case *tg.ContactsBlockedSlice:
			pageCount = len(res.Blocked)
			for _, peer := range res.Blocked {
				if p, ok := peer.PeerID.(*tg.PeerUser); ok {
					blocked[p.UserID] = true
				}
			}
		default:
			pageCount = 0
		}

		if pageCount < limit {
			break
		}
		offset += pageCount
	}

	r.storeBlockedUsers(blocked)
	return cloneBlockedUsers(blocked)
}

func (r *ChatRepository) getCachedBlockedUsers(requireFresh bool) (map[int64]bool, bool) {
	r.blockedUsersMu.RLock()
	defer r.blockedUsersMu.RUnlock()

	if len(r.blockedUsers) == 0 {
		return nil, false
	}
	if requireFresh && time.Since(r.blockedUsersAt) > blockedUsersCacheTTL {
		return nil, false
	}
	return cloneBlockedUsers(r.blockedUsers), true
}

func (r *ChatRepository) storeBlockedUsers(blocked map[int64]bool) {
	r.blockedUsersMu.Lock()
	defer r.blockedUsersMu.Unlock()

	r.blockedUsers = cloneBlockedUsers(blocked)
	r.blockedUsersAt = time.Now()
}

func cloneBlockedUsers(src map[int64]bool) map[int64]bool {
	if len(src) == 0 {
		return make(map[int64]bool)
	}
	cloned := make(map[int64]bool, len(src))
	for id, blocked := range src {
		cloned[id] = blocked
	}
	return cloned
}

// getDialogsForPeerFolder fetches all dialogs from one peer folder with pagination.
func (r *ChatRepository) getDialogsForPeerFolder(ctx context.Context, api *tg.Client, peerFolderID int32, blockedUsers map[int64]bool) ([]entity.Chat, error) {
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
		if offsetDate != 0 || offsetID != 0 || inputPeerKey(offsetPeer) != "" {
			req.ExcludePinned = true
		}

		if peerFolderID == archivePeerFolderID {
			req.FolderID = int(archivePeerFolderID)
		}

		dialogsCtx, cancel := context.WithTimeout(ctx, dialogsRPCTimeout)
		result, err := api.MessagesGetDialogs(dialogsCtx, req)
		cancel()
		if err != nil {
			return nil, err
		}

		var dialogs []tg.DialogClass
		var messages []tg.MessageClass
		var tgChats []tg.ChatClass
		var users []tg.UserClass
		var totalCount int
		noChanges := false

		switch res := result.(type) {
		case *tg.MessagesDialogs:
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
			noChanges = true
		}

		if noChanges {
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
		pageUserMap := buildUserMap(users)
		pageChannelMap := buildChannelMap(tgChats)
		nextOffsetDate, nextOffsetID, nextOffsetPeer, ok := nextDialogsPageOffset(dialogs, messages, pageUserMap, pageChannelMap)
		if !ok {
			break
		}

		if nextOffsetDate == offsetDate && nextOffsetID == offsetID && inputPeerKey(nextOffsetPeer) == inputPeerKey(offsetPeer) {
			break
		}

		offsetDate = nextOffsetDate
		offsetID = nextOffsetID
		offsetPeer = nextOffsetPeer
	}

	// Process accumulated results
	return r.processDialogs(allDialogs, allMessages, allTgChats, allUsers, blockedUsers, peerFolderID)
}

// processDialogs processes dialogs and extracts chat entities
func (r *ChatRepository) processDialogs(dialogs []tg.DialogClass, messages []tg.MessageClass, tgChats []tg.ChatClass, users []tg.UserClass, blockedUsers map[int64]bool, sourceFolderID int32) ([]entity.Chat, error) {
	// Build message date map (typed peer -> last message date)
	msgDateMap := make(map[string]int)
	for _, m := range messages {
		if peerKey, msgDate, _, ok := messagePeerAndMeta(m); ok {
			if existing, ok := msgDateMap[peerKey]; !ok || msgDate > existing {
				msgDateMap[peerKey] = msgDate
			}
		}
	}

	userMap := buildUserMap(users)
	basicChatMap := buildBasicChatMap(tgChats)
	channelMap := buildChannelMap(tgChats)
	nowUnix := int(time.Now().Unix())

	// Extract chats from dialogs in order (with deduplication)
	chats := make([]entity.Chat, 0, len(dialogs))
	seenKeys := make(map[string]bool)
	pinOrder := 0
	for _, d := range dialogs {
		dialog, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}

		peerRef, ok := peerRefFromPeer(dialog.Peer, userMap, channelMap)
		if !ok {
			continue
		}

		peerKey := peerRef.Key()

		// Skip duplicates
		if seenKeys[peerKey] {
			continue
		}
		seenKeys[peerKey] = true

		lastMsgDate := msgDateMap[peerKey]
		isPinned := dialog.Pinned
		isArchived := sourceFolderID == archivePeerFolderID
		if folderID, ok := dialog.GetFolderID(); ok {
			isArchived = folderID == int(archivePeerFolderID)
		}
		muteUntil, hasMuteUntil := dialog.NotifySettings.GetMuteUntil()
		isMuted := hasMuteUntil && muteUntil > nowUnix
		unreadCount := dialog.UnreadCount
		unreadMarked := dialog.UnreadMark

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
				Peer:            peerRef,
				Title:           name,
				Type:            entity.ChatTypePrivate,
				Username:        user.Username,
				LastMessageDate: lastMsgDate,
				IsContact:       user.Contact,
				IsBot:           user.Bot,
				IsArchived:      isArchived,
				IsMuted:         isMuted,
				UnreadCount:     unreadCount,
				UnreadMarked:    unreadMarked,
				IsPinned:        isPinned,
				PinOrder:        currentPinOrder,
				Order:           currentOrder,
				CanWrite:        canWrite,
			})

		case *tg.PeerChat:
			chat, ok := basicChatMap[peer.ChatID]
			if !ok {
				continue
			}

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
				Peer:            peerRef,
				Title:           chat.Title,
				Type:            entity.ChatTypeGroup,
				LastMessageDate: lastMsgDate,
				IsArchived:      isArchived,
				IsMuted:         isMuted,
				UnreadCount:     unreadCount,
				UnreadMarked:    unreadMarked,
				IsPinned:        isPinned,
				PinOrder:        currentPinOrder,
				Order:           currentOrder,
				CanWrite:        canWrite,
			})

		case *tg.PeerChannel:
			channel, ok := channelMap[peer.ChannelID]
			if !ok {
				continue
			}

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
				Peer:            peerRef,
				Title:           channel.Title,
				Type:            chatType,
				Username:        channel.Username,
				LastMessageDate: lastMsgDate,
				IsArchived:      isArchived,
				IsMuted:         isMuted,
				UnreadCount:     unreadCount,
				UnreadMarked:    unreadMarked,
				IsPinned:        isPinned,
				PinOrder:        currentPinOrder,
				Order:           currentOrder,
				CanWrite:        canWrite,
			})
		}
	}

	return chats, nil
}

func mergeChatInventories(mainChats []entity.Chat, archiveChats []entity.Chat) []entity.Chat {
	merged := make([]entity.Chat, 0, len(mainChats)+len(archiveChats))
	seen := make(map[string]bool, len(mainChats)+len(archiveChats))
	nextOrder := 0

	appendChats := func(chats []entity.Chat) {
		for _, chat := range chats {
			key := chat.UniqueKey()
			if seen[key] {
				continue
			}
			seen[key] = true
			chat.Order = nextOrder
			nextOrder++
			merged = append(merged, chat)
		}
	}

	appendChats(mainChats)
	appendChats(archiveChats)
	return merged
}

func buildUserMap(users []tg.UserClass) map[int64]*tg.User {
	userMap := make(map[int64]*tg.User, len(users))
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}
	return userMap
}

func buildBasicChatMap(chats []tg.ChatClass) map[int64]*tg.Chat {
	chatMap := make(map[int64]*tg.Chat)
	for _, c := range chats {
		if chat, ok := c.(*tg.Chat); ok {
			chatMap[chat.ID] = chat
		}
	}
	return chatMap
}

func buildChannelMap(chats []tg.ChatClass) map[int64]*tg.Channel {
	channelMap := make(map[int64]*tg.Channel)
	for _, c := range chats {
		if channel, ok := c.(*tg.Channel); ok {
			channelMap[channel.ID] = channel
		}
	}
	return channelMap
}

func nextDialogsPageOffset(dialogs []tg.DialogClass, messages []tg.MessageClass, userMap map[int64]*tg.User, channelMap map[int64]*tg.Channel) (int, int, tg.InputPeerClass, bool) {
	for i := len(dialogs) - 1; i >= 0; i-- {
		dialog, ok := dialogs[i].(*tg.Dialog)
		if !ok || dialog.TopMessage == 0 {
			continue
		}

		nextOffsetPeer := inputPeerFromDialogPeer(dialog.Peer, userMap, channelMap)
		if inputPeerKey(nextOffsetPeer) == "" {
			continue
		}

		nextOffsetDate, _ := dialogTopMessageDate(dialog.Peer, dialog.TopMessage, messages)
		return nextOffsetDate, dialog.TopMessage, nextOffsetPeer, true
	}

	return 0, 0, &tg.InputPeerEmpty{}, false
}

func dialogTopMessageDate(peer tg.PeerClass, topMessageID int, messages []tg.MessageClass) (int, bool) {
	targetPeerKey := peerKey(peer)
	if targetPeerKey == "" || topMessageID == 0 {
		return 0, false
	}

	fallbackDate := 0
	fallbackFound := false
	for _, message := range messages {
		msgPeerKey, msgDate, msgID, ok := messagePeerAndMeta(message)
		if !ok || msgPeerKey != targetPeerKey {
			continue
		}
		if msgID == topMessageID {
			return msgDate, true
		}
		if !fallbackFound || msgDate > fallbackDate {
			fallbackDate = msgDate
			fallbackFound = true
		}
	}

	return fallbackDate, fallbackFound
}

func messagePeerAndMeta(message tg.MessageClass) (string, int, int, bool) {
	switch msg := message.(type) {
	case *tg.Message:
		return peerKey(msg.PeerID), msg.Date, msg.ID, true
	case *tg.MessageService:
		return peerKey(msg.PeerID), msg.Date, msg.ID, true
	default:
		return "", 0, 0, false
	}
}
