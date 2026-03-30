package telegram

import "time"

const (
	authRPCTimeout         = 45 * time.Second
	profileRPCTimeout      = 20 * time.Second
	dialogsRPCTimeout      = 45 * time.Second
	blockedUsersRPCTimeout = 30 * time.Second
	sendMediaRPCTimeout    = 2 * time.Minute
	uploadOperationTimeout = 30 * time.Minute
	cacheMaxAge            = 15 * time.Minute
)
