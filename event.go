package gogent

type ChatEventType string

const (
	ChatEventTypeMessageAdded   ChatEventType = "message_added"
	ChatEventTypeMessageUpdated ChatEventType = "message_updated"
)

type ChatEvent struct {
	ID        string        `json:"id"`
	ChatID    string        `json:"chat_id"`
	Type      ChatEventType `json:"type"`
	MessageID *string       `json:"message_id,omitempty"`
}

type ChatEventBroadcaster interface {
	Broadcast(event ChatEvent) error
}

// NopBroadcaster discards all chat events.
type NopBroadcaster struct{}

func (NopBroadcaster) Broadcast(ChatEvent) error { return nil }

// ChannelBroadcaster broadcasts chat events to a channel.
type ChannelBroadcaster struct {
	ch chan ChatEvent
}

func NewChannelBroadcaster() *ChannelBroadcaster {
	return &ChannelBroadcaster{ch: make(chan ChatEvent)}
}

func (b *ChannelBroadcaster) Broadcast(event ChatEvent) error {
	b.ch <- event
	return nil
}

func (b *ChannelBroadcaster) Channel() <-chan ChatEvent {
	return b.ch
}
