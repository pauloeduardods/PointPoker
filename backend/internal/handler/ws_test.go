package handler_test

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pauloedsg/pointpoker/internal/hub"
)

type wsEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type wsConn struct {
	t    *testing.T
	conn *websocket.Conn
}

func dialWS(t *testing.T, srv *httptest.Server, code, token string) *wsConn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/rooms/" + code + "/ws?token=" + token
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v (resp %v)", err, resp)
	}
	t.Cleanup(func() { conn.Close() })
	return &wsConn{t: t, conn: conn}
}

// next returns the next event, failing the test after a timeout.
func (c *wsConn) next() wsEvent {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ev wsEvent
	if err := c.conn.ReadJSON(&ev); err != nil {
		c.t.Fatalf("read event: %v", err)
	}
	return ev
}

// waitFor skips events until one of the given type arrives.
func (c *wsConn) waitFor(typ string) wsEvent {
	c.t.Helper()
	for {
		if ev := c.next(); ev.Type == typ {
			return ev
		}
	}
}

// waitPresence waits for a presence event whose online set equals want.
func (c *wsConn) waitPresence(want ...string) {
	c.t.Helper()
	slices.Sort(want)
	for {
		ev := c.waitFor(hub.EventPresence)
		var p hub.PresencePayload
		_ = json.Unmarshal(ev.Payload, &p)
		if slices.Equal(p.Online, want) {
			return
		}
	}
}

func TestWebSocketEvents(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(e.router)
	defer srv.Close()

	host := e.createRoom("Room", "Host")
	code := host.Room.Code
	other := e.createRoom("Other", "Other")

	hostWS := dialWS(t, srv, strings.ToLower(code), host.SessionToken)
	hostWS.waitPresence(host.Participant.ID)
	otherWS := dialWS(t, srv, other.Room.Code, other.SessionToken)
	otherWS.waitPresence(other.Participant.ID)

	// participant_joined carries the participant without its token.
	guest := e.joinRoom(code, "Guest")
	ev := hostWS.waitFor(hub.EventParticipantJoined)
	if strings.Contains(string(ev.Payload), guest.SessionToken) {
		t.Errorf("participant_joined leaks token: %s", ev.Payload)
	}
	var joined struct {
		Participant struct {
			ID string `json:"id"`
		} `json:"participant"`
	}
	_ = json.Unmarshal(ev.Payload, &joined)
	if joined.Participant.ID != guest.Participant.ID {
		t.Errorf("participant_joined = %s", ev.Payload)
	}

	// Presence reflects both connections; GET room reports online flags.
	guestWS := dialWS(t, srv, code, guest.SessionToken)
	hostWS.waitPresence(host.Participant.ID, guest.Participant.ID)
	guestWS.waitPresence(host.Participant.ID, guest.Participant.ID)
	r := e.expect("GET", "/api/rooms/"+code, "", nil, 200)
	if strings.Count(string(r.Body), `"online":true`) != 2 {
		t.Errorf("room = %s", r.Body)
	}

	round := e.startRound(code, host.SessionToken, "Story")
	ev = guestWS.waitFor(hub.EventRoundStarted)
	if !strings.Contains(string(ev.Payload), round.ID) {
		t.Errorf("round_started = %s", ev.Payload)
	}

	e.expect("POST", "/api/rooms/"+code+"/rounds/"+round.ID+"/vote", guest.SessionToken, map[string]string{"value": "13"}, 200)
	ev = hostWS.waitFor(hub.EventVoteCast)
	if string(ev.Payload) != `{"participant_id":"`+guest.Participant.ID+`"}` {
		t.Errorf("vote_cast = %s (must not include the value)", ev.Payload)
	}

	e.expect("POST", "/api/rooms/"+code+"/rounds/"+round.ID+"/reveal", host.SessionToken, nil, 200)
	ev = guestWS.waitFor(hub.EventVotesRevealed)
	if !strings.Contains(string(ev.Payload), `"value":"13"`) || !strings.Contains(string(ev.Payload), round.ID) {
		t.Errorf("votes_revealed = %s", ev.Payload)
	}

	e.expect("POST", "/api/rooms/"+code+"/rounds/"+round.ID+"/reset", host.SessionToken, nil, 200)
	ev = guestWS.waitFor(hub.EventRoundReset)
	if !strings.Contains(string(ev.Payload), `"status":"voting"`) {
		t.Errorf("round_reset = %s", ev.Payload)
	}

	// Host leaves: guest is promoted, host's socket is closed.
	e.expect("DELETE", "/api/rooms/"+code+"/participants/me", host.SessionToken, nil, 204)
	ev = guestWS.waitFor(hub.EventParticipantLeft)
	want := `{"new_host_id":"` + guest.Participant.ID + `","participant_id":"` + host.Participant.ID + `"}`
	if string(ev.Payload) != want {
		t.Errorf("participant_left = %s, want %s", ev.Payload, want)
	}
	_ = hostWS.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := hostWS.conn.ReadMessage(); err != nil {
			break // closed by the server
		}
	}

	// Guest disconnects: the room's hub is released.
	guestWS.conn.Close()
	deadline := time.Now().Add(3 * time.Second)
	for e.hubs.ClientCount(code) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("room hub still has %d clients", e.hubs.ClientCount(code))
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The other room never saw any of this.
	_ = otherWS.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, msg, err := otherWS.conn.ReadMessage(); err == nil {
		t.Errorf("other room received %s", msg)
	}
	if e.hubs.RoomCount() != 1 {
		t.Errorf("RoomCount = %d, want 1", e.hubs.RoomCount())
	}
}

func TestWebSocketParticipantLeftWithoutHostChange(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(e.router)
	defer srv.Close()

	host := e.createRoom("Room", "Host")
	guest := e.joinRoom(host.Room.Code, "Guest")
	hostWS := dialWS(t, srv, host.Room.Code, host.SessionToken)
	hostWS.waitPresence(host.Participant.ID)

	e.expect("DELETE", "/api/rooms/"+host.Room.Code+"/participants/me", guest.SessionToken, nil, 204)
	ev := hostWS.waitFor(hub.EventParticipantLeft)
	if want := `{"new_host_id":null,"participant_id":"` + guest.Participant.ID + `"}`; string(ev.Payload) != want {
		t.Errorf("participant_left = %s, want %s", ev.Payload, want)
	}
}
