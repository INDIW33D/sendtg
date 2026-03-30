package cache

import (
	"testing"

	"sendtg/internal/domain/entity"
)

func TestDialogCacheResetsOnAccountChange(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewDialogCache(dir)
	if err != nil {
		t.Fatalf("NewDialogCache returned error: %v", err)
	}

	if err := cache.SetChats(1001, []entity.Chat{{Title: "Alice"}}); err != nil {
		t.Fatalf("SetChats returned error: %v", err)
	}
	if err := cache.SetFolders(1001, []entity.Folder{{Title: "Work"}}); err != nil {
		t.Fatalf("SetFolders returned error: %v", err)
	}

	if got := cache.GetAccountID(); got != 1001 {
		t.Fatalf("expected account id 1001, got %d", got)
	}
	if len(cache.GetChats()) != 1 || len(cache.GetFolders()) != 1 {
		t.Fatal("expected chats and folders to be cached for first account")
	}

	if err := cache.SetFolders(2002, []entity.Folder{{Title: "Personal"}}); err != nil {
		t.Fatalf("SetFolders returned error: %v", err)
	}

	if got := cache.GetAccountID(); got != 2002 {
		t.Fatalf("expected account id 2002, got %d", got)
	}
	if len(cache.GetChats()) != 0 {
		t.Fatal("expected chats from previous account to be cleared")
	}
	if len(cache.GetFolders()) != 1 || cache.GetFolders()[0].Title != "Personal" {
		t.Fatal("expected folders for new account to replace previous cache")
	}
}
