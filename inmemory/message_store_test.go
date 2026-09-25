package inmemory

import (
	"context"
	"sync"
	"testing"

	"github.com/cgund98/gogent"
)

func TestMessageStoreConcurrentLoadAndWrite(t *testing.T) {
	store := NewMessageStore()
	ctx := context.Background()
	const chatID = "chat"
	if err := store.AddMessages(ctx, chatID, gogent.NewUserMessage("git status")); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			message := gogent.NewAssistantMessage("running")
			if err := store.AddMessages(ctx, chatID, message); err != nil {
				t.Error(err)
				return
			}
			message.Content = "done"
			if err := store.UpdateMessage(ctx, chatID, message.ID, message); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if _, err := store.Load(ctx, chatID); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()
}
