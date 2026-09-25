package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/cgund98/gogent"
)

// MessageStore keeps chat transcripts in memory.
// The agent writes from its run goroutine while the UI loads the same chat, so every method takes the lock.
type MessageStore struct {
	mu       sync.Mutex
	messages map[string][]gogent.Message
}

func NewMessageStore() *MessageStore {
	return &MessageStore{messages: make(map[string][]gogent.Message)}
}

func (s *MessageStore) Load(_ context.Context, chatID string) ([]gogent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]gogent.Message(nil), s.messages[chatID]...), nil
}

func (s *MessageStore) GetMessage(_ context.Context, chatID string, messageID string) (gogent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, message := range s.messages[chatID] {
		if message.ID == messageID {
			return message, nil
		}
	}
	return gogent.Message{}, fmt.Errorf("message %s not found", messageID)
}

func (s *MessageStore) AddMessages(_ context.Context, chatID string, messages ...gogent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[chatID] = append(s.messages[chatID], messages...)
	return nil
}

func (s *MessageStore) UpdateMessage(_ context.Context, chatID string, messageID string, message gogent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.messages[chatID] {
		if existing.ID == messageID {
			s.messages[chatID][i] = message
			return nil
		}
	}
	return fmt.Errorf("message %s not found", messageID)
}

func (s *MessageStore) DeleteMessage(_ context.Context, chatID string, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, message := range s.messages[chatID] {
		if message.ID == messageID {
			s.messages[chatID] = append(s.messages[chatID][:i], s.messages[chatID][i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("message %s not found", messageID)
}

func (s *MessageStore) DeleteAllMessages(_ context.Context, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, chatID)
	return nil
}
