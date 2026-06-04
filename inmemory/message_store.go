package inmemory

import (
	"context"
	"fmt"

	"github.com/cgund98/gogent"
)

type MessageStore struct {
	messages map[string][]gogent.Message
}

func NewMessageStore() *MessageStore {
	return &MessageStore{messages: make(map[string][]gogent.Message)}
}

func (s *MessageStore) Load(_ context.Context, chatID string) ([]gogent.Message, error) {
	return append([]gogent.Message(nil), s.messages[chatID]...), nil
}

func (s *MessageStore) GetMessage(_ context.Context, chatID string, messageID string) (gogent.Message, error) {
	for _, message := range s.messages[chatID] {
		if message.ID == messageID {
			return message, nil
		}
	}
	return gogent.Message{}, fmt.Errorf("message %s not found", messageID)
}

func (s *MessageStore) AddMessages(_ context.Context, chatID string, messages ...gogent.Message) error {
	s.messages[chatID] = append(s.messages[chatID], messages...)
	return nil
}

func (s *MessageStore) UpdateMessage(_ context.Context, chatID string, messageID string, message gogent.Message) error {
	for i, existing := range s.messages[chatID] {
		if existing.ID == messageID {
			s.messages[chatID][i] = message
			return nil
		}
	}
	return fmt.Errorf("message %s not found", messageID)
}

func (s *MessageStore) DeleteMessage(_ context.Context, chatID string, messageID string) error {
	for i, message := range s.messages[chatID] {
		if message.ID == messageID {
			s.messages[chatID] = append(s.messages[chatID][:i], s.messages[chatID][i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("message %s not found", messageID)
}

func (s *MessageStore) DeleteAllMessages(_ context.Context, chatID string) error {
	delete(s.messages, chatID)
	return nil
}
