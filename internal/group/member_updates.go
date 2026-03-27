package group

import "sync"

type MemberUpdateBroker struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[string]map[int]chan *MemberView
}

func NewMemberUpdateBroker() *MemberUpdateBroker {
	return &MemberUpdateBroker{
		subscribers: make(map[string]map[int]chan *MemberView),
	}
}

func (b *MemberUpdateBroker) Subscribe(groupID string) (<-chan *MemberView, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscribers == nil {
		b.subscribers = make(map[string]map[int]chan *MemberView)
	}
	if b.subscribers[groupID] == nil {
		b.subscribers[groupID] = make(map[int]chan *MemberView)
	}
	id := b.nextID
	b.nextID++
	ch := make(chan *MemberView, 16)
	b.subscribers[groupID][id] = ch

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		groupSubscribers := b.subscribers[groupID]
		if groupSubscribers == nil {
			return
		}
		if subscriber, ok := groupSubscribers[id]; ok {
			delete(groupSubscribers, id)
			close(subscriber)
		}
		if len(groupSubscribers) == 0 {
			delete(b.subscribers, groupID)
		}
	}
}

func (b *MemberUpdateBroker) Publish(groupID string, item *MemberView) {
	if item == nil {
		return
	}

	b.mu.RLock()
	groupSubscribers := b.subscribers[groupID]
	subscribers := make([]chan *MemberView, 0, len(groupSubscribers))
	for _, subscriber := range groupSubscribers {
		subscribers = append(subscribers, subscriber)
	}
	b.mu.RUnlock()

	for _, subscriber := range subscribers {
		cloned := cloneMemberView(item)
		select {
		case subscriber <- cloned:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- cloned:
			default:
			}
		}
	}
}

func cloneMemberView(item *MemberView) *MemberView {
	if item == nil {
		return nil
	}
	cloned := *item
	if item.SubscriptionSourceID != nil {
		sourceID := *item.SubscriptionSourceID
		cloned.SubscriptionSourceID = &sourceID
	}
	if item.LastLatencyMS != nil {
		latency := *item.LastLatencyMS
		cloned.LastLatencyMS = &latency
	}
	if item.LastCheckedAt != nil {
		checkedAt := *item.LastCheckedAt
		cloned.LastCheckedAt = &checkedAt
	}
	return &cloned
}
