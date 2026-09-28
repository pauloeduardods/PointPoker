package hub

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"
)

// newTestClient returns a client without a connection; tests read its send channel directly.
func newTestClient(room, participant string) *Client {
	return NewClient(nil, room, participant)
}

// drain returns every message currently queued for c, decoded.
func drain(t *testing.T, c *Client) []WSMessage {
	t.Helper()
	var out []WSMessage
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return out
			}
			var msg WSMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("bad message %s: %v", data, err)
			}
			out = append(out, msg)
		default:
			return out
		}
	}
}

func presenceOf(t *testing.T, msg WSMessage) []string {
	t.Helper()
	if msg.Type != EventPresence {
		t.Fatalf("type = %q, want presence", msg.Type)
	}
	raw, _ := json.Marshal(msg.Payload)
	var p PresencePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return p.Online
}

func isClosed(c *Client) bool {
	for {
		select {
		case _, ok := <-c.send:
			if !ok {
				return true
			}
		default:
			return false
		}
	}
}

func TestRegisterBroadcastsPresence(t *testing.T) {
	m := NewHubManager()
	a := newTestClient("ROOM", "p1")
	m.Register(a)

	msgs := drain(t, a)
	if len(msgs) != 1 || !slices.Equal(presenceOf(t, msgs[0]), []string{"p1"}) {
		t.Fatalf("messages after first register = %+v", msgs)
	}

	b := newTestClient("ROOM", "p2")
	m.Register(b)
	for _, c := range []*Client{a, b} {
		msgs := drain(t, c)
		if len(msgs) != 1 || !slices.Equal(presenceOf(t, msgs[0]), []string{"p1", "p2"}) {
			t.Fatalf("messages = %+v", msgs)
		}
	}
	if got := m.ClientCount("ROOM"); got != 2 {
		t.Errorf("ClientCount = %d", got)
	}
}

func TestPresenceDedupesParticipants(t *testing.T) {
	m := NewHubManager()
	tab1 := newTestClient("ROOM", "p1")
	tab2 := newTestClient("ROOM", "p1")
	other := newTestClient("ROOM", "p2")
	m.Register(tab1)
	m.Register(tab2)
	m.Register(other)
	if got := m.Online("ROOM"); !slices.Equal(got, []string{"p1", "p2"}) {
		t.Fatalf("Online = %v", got)
	}
	drain(t, other)

	// p1 stays online while one tab remains.
	m.Unregister(tab1)
	msgs := drain(t, other)
	if len(msgs) != 1 || !slices.Equal(presenceOf(t, msgs[0]), []string{"p1", "p2"}) {
		t.Fatalf("after closing one tab: %+v", msgs)
	}
	m.Unregister(tab2)
	msgs = drain(t, other)
	if len(msgs) != 1 || !slices.Equal(presenceOf(t, msgs[0]), []string{"p2"}) {
		t.Fatalf("after closing both tabs: %+v", msgs)
	}
}

func TestUnregisterIsIdempotentAndRemovesEmptyHub(t *testing.T) {
	m := NewHubManager()
	c := newTestClient("ROOM", "p1")
	m.Register(c)
	m.Unregister(c)
	m.Unregister(c) // must not panic (double close)
	if !isClosed(c) {
		t.Error("send channel not closed")
	}
	if n := m.RoomCount(); n != 0 {
		t.Errorf("RoomCount = %d, want 0 (hub leaked)", n)
	}
	if got := m.Online("ROOM"); got == nil || len(got) != 0 {
		t.Errorf("Online = %#v, want empty non-nil", got)
	}

	// The room can be used again after its hub was removed.
	d := newTestClient("ROOM", "p2")
	m.Register(d)
	if m.ClientCount("ROOM") != 1 {
		t.Error("re-register failed")
	}
}

func TestBroadcastIsScopedToRoom(t *testing.T) {
	m := NewHubManager()
	a := newTestClient("A", "p1")
	b := newTestClient("B", "p2")
	m.Register(a)
	m.Register(b)
	drain(t, a)
	drain(t, b)

	m.Broadcast("A", WSMessage{Type: EventVoteCast, Payload: map[string]string{"participant_id": "p1"}})
	if msgs := drain(t, a); len(msgs) != 1 || msgs[0].Type != EventVoteCast {
		t.Errorf("room A got %+v", msgs)
	}
	if msgs := drain(t, b); len(msgs) != 0 {
		t.Errorf("room B got %+v", msgs)
	}
	// Broadcasting to a room without clients is a no-op and creates no hub.
	m.Broadcast("EMPTY", WSMessage{Type: EventVoteCast})
	if m.RoomCount() != 2 {
		t.Errorf("RoomCount = %d", m.RoomCount())
	}
}

func TestSlowClientIsDroppedWithoutBlocking(t *testing.T) {
	m := NewHubManager()
	slow := newTestClient("ROOM", "slow")
	fast := newTestClient("ROOM", "fast")
	m.Register(slow)
	m.Register(fast)
	drain(t, fast)

	// Never drain slow; its buffer fills up and it must be dropped.
	for i := range sendBufferSize + 5 {
		m.Broadcast("ROOM", WSMessage{Type: EventVoteCast, Payload: i})
		drain(t, fast)
	}
	if got := m.Online("ROOM"); !slices.Equal(got, []string{"fast"}) {
		t.Errorf("Online = %v, want [fast]", got)
	}
	// The pump's eventual Unregister must be a harmless no-op.
	m.Unregister(slow)
	m.Unregister(fast)
	if m.RoomCount() != 0 {
		t.Errorf("RoomCount = %d", m.RoomCount())
	}
}

func TestDisconnectParticipant(t *testing.T) {
	m := NewHubManager()
	tab1 := newTestClient("ROOM", "leaver")
	tab2 := newTestClient("ROOM", "leaver")
	stay := newTestClient("ROOM", "stay")
	for _, c := range []*Client{tab1, tab2, stay} {
		m.Register(c)
	}
	drain(t, stay)
	m.DisconnectParticipant("ROOM", "leaver")
	if !isClosed(tab1) || !isClosed(tab2) {
		t.Error("leaver's connections not closed")
	}
	msgs := drain(t, stay)
	if len(msgs) != 1 || !slices.Equal(presenceOf(t, msgs[0]), []string{"stay"}) {
		t.Errorf("messages = %+v", msgs)
	}
	m.DisconnectParticipant("NOROOM", "x") // no-op
}

func TestCloseDisconnectsEveryone(t *testing.T) {
	m := NewHubManager()
	a := newTestClient("A", "p1")
	m.Register(a)
	m.Close()
	if !isClosed(a) {
		t.Error("client not closed")
	}
	late := newTestClient("A", "p2")
	m.Register(late)
	if !isClosed(late) || m.RoomCount() != 0 {
		t.Error("registration accepted after Close")
	}
	m.Unregister(a) // no panic
}

// TestConcurrentUse exercises every method concurrently; run with -race.
func TestConcurrentUse(t *testing.T) {
	m := NewHubManager()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			room := fmt.Sprintf("R%d", i%3)
			for j := range 50 {
				c := newTestClient(room, fmt.Sprintf("p%d", j%5))
				m.Register(c)
				m.Broadcast(room, WSMessage{Type: EventVoteCast, Payload: j})
				_ = m.Online(room)
				if j%7 == 0 {
					m.DisconnectParticipant(room, c.participantID)
				}
				m.Unregister(c)
			}
		})
	}
	wg.Wait()
	if n := m.RoomCount(); n != 0 {
		t.Errorf("RoomCount = %d after all clients left", n)
	}
}
