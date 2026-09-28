package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/handler"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/memstore"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/service"
)

func init() { gin.SetMode(gin.TestMode) }

type fakeDB struct{ err error }

func (f fakeDB) PingContext(context.Context) error { return f.err }

type env struct {
	t      *testing.T
	store  *memstore.Store
	hubs   *hub.HubManager
	db     *fakeDB
	router http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st := memstore.New()
	e := &env{t: t, store: st, hubs: hub.NewHubManager(), db: &fakeDB{}}
	e.router = handler.NewRouter(handler.Deps{
		Rooms:       service.NewRoomService(st),
		Voting:      service.NewVotingService(st, st),
		Hubs:        e.hubs,
		DB:          e.db,
		CORSOrigins: []string{"http://localhost:5173"},
	})
	return e
}

type response struct {
	Code int
	Body []byte
}

func (r response) decode(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

func (e *env) do(method, path, token string, body any) response {
	e.t.Helper()
	var buf *bytes.Reader
	switch b := body.(type) {
	case nil:
		buf = bytes.NewReader(nil)
	case string:
		buf = bytes.NewReader([]byte(b))
	default:
		data, _ := json.Marshal(b)
		buf = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Session-Token", token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return response{rec.Code, rec.Body.Bytes()}
}

func (e *env) expect(method, path, token string, body any, status int) response {
	e.t.Helper()
	r := e.do(method, path, token, body)
	if r.Code != status {
		e.t.Fatalf("%s %s: status = %d, want %d; body = %s", method, path, r.Code, status, r.Body)
	}
	if status >= 400 {
		var er struct{ Error string }
		r.decode(e.t, &er)
		if er.Error == "" {
			e.t.Fatalf("%s %s: error body without message: %s", method, path, r.Body)
		}
	}
	return r
}

type sessionResp struct {
	Room         model.Room        `json:"room"`
	SessionToken string            `json:"session_token"`
	Participant  model.Participant `json:"participant"`
}

func (e *env) createRoom(name, display string) sessionResp {
	e.t.Helper()
	var s sessionResp
	e.expect("POST", "/api/rooms", "", map[string]string{"name": name, "display_name": display}, 201).decode(e.t, &s)
	return s
}

func (e *env) joinRoom(code, display string) sessionResp {
	e.t.Helper()
	var s sessionResp
	e.expect("POST", "/api/rooms/"+code+"/join", "", map[string]string{"display_name": display}, 200).decode(e.t, &s)
	return s
}

func (e *env) startRound(code, token, title string) model.VotingRound {
	e.t.Helper()
	var out struct{ Round model.VotingRound }
	e.expect("POST", "/api/rooms/"+code+"/rounds", token, map[string]string{"story_title": title}, 201).decode(e.t, &out)
	return out.Round
}

func TestHealth(t *testing.T) {
	e := newEnv(t)
	r := e.expect("GET", "/api/health", "", nil, 200)
	if !strings.Contains(string(r.Body), `"status":"ok"`) {
		t.Errorf("body = %s", r.Body)
	}
	e.db.err = errors.New("down")
	e.expect("GET", "/api/health", "", nil, 503)
}

func TestUnknownRouteIsJSON(t *testing.T) {
	e := newEnv(t)
	e.expect("GET", "/api/nope", "", nil, 404)
}

func TestCORS(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest("OPTIONS", "/api/rooms", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "DELETE")
	req.Header.Set("Access-Control-Request-Headers", "X-Session-Token")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allowed origin = %q", got)
	}

	req = httptest.NewRequest("OPTIONS", "/api/rooms", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unexpected allowed origin %q", got)
	}
}

func TestSameOriginRequestBypassesCORSAllowList(t *testing.T) {
	e := newEnv(t)
	body := strings.NewReader(`{"name":"Sprint","display_name":"Ana"}`)
	req := httptest.NewRequest("POST", "http://localhost:8000/api/rooms", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8000")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("same-origin POST status = %d, body = %s", rec.Code, rec.Body)
	}

	req = httptest.NewRequest("POST", "http://localhost:8000/api/rooms", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("cross-origin POST from unknown origin status = %d", rec.Code)
	}
}

func TestCreateRoom(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name   string
		body   any
		status int
	}{
		{"ok", map[string]string{"name": " Sprint ", "display_name": " Alice "}, 201},
		{"malformed json", "{", 400},
		{"missing fields", map[string]string{}, 400},
		{"blank name", map[string]string{"name": "  ", "display_name": "A"}, 400},
		{"long display name", map[string]string{"name": "R", "display_name": strings.Repeat("a", 51)}, 400},
		{"long room name", map[string]string{"name": strings.Repeat("a", 101), "display_name": "A"}, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.t = t
			r := e.expect("POST", "/api/rooms", "", tt.body, tt.status)
			if tt.status != 201 {
				return
			}
			var s sessionResp
			r.decode(t, &s)
			if s.Room.Name != "Sprint" || s.Participant.DisplayName != "Alice" || !s.Participant.IsHost || s.SessionToken == "" {
				t.Errorf("response = %s", r.Body)
			}
			// The token appears exactly once: top-level, not inside participant.
			if n := strings.Count(string(r.Body), s.SessionToken); n != 1 {
				t.Errorf("session token appears %d times in %s", n, r.Body)
			}
		})
	}
}

func TestJoinAndGetRoom(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	lower := strings.ToLower(host.Room.Code)

	guest := e.joinRoom(lower, " Guest ")
	if guest.Room.ID != host.Room.ID || guest.Participant.IsHost || guest.Participant.DisplayName != "Guest" {
		t.Errorf("join = %+v", guest)
	}
	if strings.Count(string(e.do("POST", "/api/rooms/"+host.Room.Code+"/join", "", map[string]string{"display_name": "X"}).Body), "session_token") != 1 {
		t.Error("session_token should appear once in join response")
	}
	e.expect("POST", "/api/rooms/ZZZZZZ/join", "", map[string]string{"display_name": "X"}, 404)
	e.expect("POST", "/api/rooms/"+host.Room.Code+"/join", "", map[string]string{"display_name": ""}, 400)
	e.expect("POST", "/api/rooms/"+host.Room.Code+"/join", "", "nope", 400)

	r := e.expect("GET", "/api/rooms/"+lower, "", nil, 200)
	if strings.Contains(string(r.Body), "session_token") || strings.Contains(string(r.Body), host.SessionToken) {
		t.Errorf("room response leaks tokens: %s", r.Body)
	}
	var room struct {
		Room         model.Room
		Participants []struct {
			ID     string `json:"id"`
			IsHost bool   `json:"is_host"`
			Online *bool  `json:"online"`
		}
	}
	r.decode(t, &room)
	if room.Room.Code != host.Room.Code || len(room.Participants) != 3 {
		t.Fatalf("room = %s", r.Body)
	}
	for _, p := range room.Participants {
		if p.Online == nil || *p.Online {
			t.Errorf("participant %s online = %v, want false", p.ID, p.Online)
		}
	}
	e.expect("GET", "/api/rooms/ZZZZZZ", "", nil, 404)
}

func TestMe(t *testing.T) {
	e := newEnv(t)
	a := e.createRoom("A", "HostA")
	b := e.createRoom("B", "HostB")
	path := "/api/rooms/" + a.Room.Code + "/me"

	var out struct{ Participant model.Participant }
	e.expect("GET", path, a.SessionToken, nil, 200).decode(t, &out)
	if out.Participant.ID != a.Participant.ID || !out.Participant.IsHost {
		t.Errorf("me = %+v", out.Participant)
	}
	e.expect("GET", "/api/rooms/"+strings.ToLower(a.Room.Code)+"/me", a.SessionToken, nil, 200)
	e.expect("GET", path, "", nil, 401)
	e.expect("GET", path, "garbage", nil, 401)
	e.expect("GET", path, "00000000-0000-0000-0000-000000000000", nil, 401)
	e.expect("GET", path, b.SessionToken, nil, 403)
	e.expect("GET", "/api/rooms/ZZZZZZ/me", a.SessionToken, nil, 404)
}

func TestLeave(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	second := e.joinRoom(host.Room.Code, "Second")
	e.joinRoom(host.Room.Code, "Third")
	other := e.createRoom("Other", "Other")
	path := "/api/rooms/" + host.Room.Code + "/participants/me"

	e.expect("DELETE", path, "", nil, 401)
	e.expect("DELETE", path, other.SessionToken, nil, 403)
	e.expect("DELETE", "/api/rooms/ZZZZZZ/participants/me", host.SessionToken, nil, 404)

	r := e.expect("DELETE", path, host.SessionToken, nil, 204)
	if len(r.Body) != 0 {
		t.Errorf("204 with body %s", r.Body)
	}
	e.expect("GET", "/api/rooms/"+host.Room.Code+"/me", host.SessionToken, nil, 401)

	var out struct{ Participant model.Participant }
	e.expect("GET", "/api/rooms/"+host.Room.Code+"/me", second.SessionToken, nil, 200).decode(t, &out)
	if !out.Participant.IsHost {
		t.Error("earliest remaining participant was not promoted to host")
	}
	// The new host can now start a round.
	e.startRound(host.Room.Code, second.SessionToken, "After transfer")
}

func TestStartRound(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	guest := e.joinRoom(host.Room.Code, "Guest")
	other := e.createRoom("Other", "Other")
	path := "/api/rooms/" + host.Room.Code + "/rounds"
	body := map[string]string{"story_title": "Story"}

	e.expect("POST", path, "", body, 401)
	e.expect("POST", path, guest.SessionToken, body, 403)
	e.expect("POST", path, other.SessionToken, body, 403)
	e.expect("POST", "/api/rooms/ZZZZZZ/rounds", host.SessionToken, body, 404)
	e.expect("POST", path, host.SessionToken, map[string]string{"story_title": " "}, 400)
	e.expect("POST", path, host.SessionToken, map[string]string{"story_title": strings.Repeat("s", 201)}, 400)
	e.expect("POST", path, host.SessionToken, "{", 400)

	round := e.startRound(strings.ToLower(host.Room.Code), host.SessionToken, " Story ")
	if round.StoryTitle != "Story" || round.Status != model.RoundStatusVoting || round.RoomID != host.Room.ID {
		t.Errorf("round = %+v", round)
	}
}

type currentResp struct {
	Round *model.VotingRound `json:"round"`
	Votes []model.Vote       `json:"votes"`
}

func (e *env) current(code, token string) (currentResp, string) {
	e.t.Helper()
	r := e.expect("GET", "/api/rooms/"+code+"/rounds/current", token, nil, 200)
	var out currentResp
	r.decode(e.t, &out)
	return out, string(r.Body)
}

func TestCurrentRoundAndVoteHiding(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	guest := e.joinRoom(host.Room.Code, "Guest")
	code := host.Room.Code

	cur, raw := e.current(code, "")
	if cur.Round != nil || !strings.Contains(raw, `"votes":[]`) || !strings.Contains(raw, `"round":null`) {
		t.Errorf("empty room current = %s", raw)
	}
	e.expect("GET", "/api/rooms/ZZZZZZ/rounds/current", "", nil, 404)

	round := e.startRound(code, host.SessionToken, "Story")
	cur, raw = e.current(code, host.SessionToken)
	if cur.Round == nil || cur.Round.ID != round.ID || !strings.Contains(raw, `"votes":[]`) {
		t.Errorf("current = %s", raw)
	}

	votePath := "/api/rooms/" + code + "/rounds/" + round.ID + "/vote"
	e.expect("POST", votePath, host.SessionToken, map[string]string{"value": "5"}, 200)
	e.expect("POST", votePath, guest.SessionToken, map[string]string{"value": "☕"}, 200)

	tests := []struct {
		name  string
		token string
		own   string // participant whose value is visible
	}{
		{"host sees own", host.SessionToken, host.Participant.ID},
		{"guest sees own", guest.SessionToken, guest.Participant.ID},
		{"anonymous sees none", "", ""},
		{"invalid token sees none", "garbage", ""},
	}
	want := map[string]string{host.Participant.ID: "5", guest.Participant.ID: "☕"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.t = t
			cur, _ := e.current(code, tt.token)
			if len(cur.Votes) != 2 {
				t.Fatalf("votes = %+v", cur.Votes)
			}
			for _, v := range cur.Votes {
				exp := ""
				if v.ParticipantID == tt.own {
					exp = want[v.ParticipantID]
				}
				if v.Value != exp {
					t.Errorf("value of %s = %q, want %q", v.ParticipantID, v.Value, exp)
				}
			}
		})
	}
	e.t = t

	e.expect("POST", "/api/rooms/"+code+"/rounds/"+round.ID+"/reveal", host.SessionToken, nil, 200)
	cur, _ = e.current(code, "")
	for _, v := range cur.Votes {
		if v.Value != want[v.ParticipantID] {
			t.Errorf("after reveal value of %s = %q", v.ParticipantID, v.Value)
		}
	}
}

func TestVote(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	guest := e.joinRoom(host.Room.Code, "Guest")
	other := e.createRoom("Other", "Other")
	round := e.startRound(host.Room.Code, host.SessionToken, "Story")
	otherRound := e.startRound(other.Room.Code, other.SessionToken, "Other story")
	base := "/api/rooms/" + host.Room.Code + "/rounds/"
	five := map[string]string{"value": "5"}

	tests := []struct {
		name, path, token string
		body              any
		status            int
	}{
		{"ok", base + round.ID + "/vote", guest.SessionToken, five, 200},
		{"coffee", base + round.ID + "/vote", guest.SessionToken, map[string]string{"value": "☕"}, 200},
		{"no token", base + round.ID + "/vote", "", five, 401},
		{"bad token", base + round.ID + "/vote", "garbage", five, 401},
		{"other room member", base + round.ID + "/vote", other.SessionToken, five, 403},
		{"invalid value", base + round.ID + "/vote", guest.SessionToken, map[string]string{"value": "4"}, 400},
		{"empty value", base + round.ID + "/vote", guest.SessionToken, map[string]string{}, 400},
		{"malformed body", base + round.ID + "/vote", guest.SessionToken, "{", 400},
		{"non-uuid round", base + "abc/vote", guest.SessionToken, five, 404},
		{"unknown round", base + "00000000-0000-0000-0000-000000000000/vote", guest.SessionToken, five, 404},
		{"round of other room", base + otherRound.ID + "/vote", guest.SessionToken, five, 404},
		{"unknown room", "/api/rooms/ZZZZZZ/rounds/" + round.ID + "/vote", guest.SessionToken, five, 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.t = t
			r := e.expect("POST", tt.path, tt.token, tt.body, tt.status)
			if tt.status == 200 {
				var out struct{ Vote model.Vote }
				r.decode(t, &out)
				if out.Vote.ParticipantID != guest.Participant.ID || out.Vote.RoundID != round.ID || out.Vote.Value == "" {
					t.Errorf("vote = %+v", out.Vote)
				}
			}
		})
	}
	e.t = t

	e.expect("POST", base+round.ID+"/reveal", host.SessionToken, nil, 200)
	e.expect("POST", base+round.ID+"/vote", guest.SessionToken, five, 409)
}

func TestRevealAndReset(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	guest := e.joinRoom(host.Room.Code, "Guest")
	other := e.createRoom("Other", "Other")
	round := e.startRound(host.Room.Code, host.SessionToken, "Story")
	otherRound := e.startRound(other.Room.Code, other.SessionToken, "Other")
	base := "/api/rooms/" + host.Room.Code + "/rounds/"
	e.expect("POST", base+round.ID+"/vote", guest.SessionToken, map[string]string{"value": "8"}, 200)

	for _, action := range []string{"reveal", "reset"} {
		t.Run(action+" auth", func(t *testing.T) {
			e.t = t
			e.expect("POST", base+round.ID+"/"+action, "", nil, 401)
			e.expect("POST", base+round.ID+"/"+action, guest.SessionToken, nil, 403)
			e.expect("POST", base+round.ID+"/"+action, other.SessionToken, nil, 403)
			e.expect("POST", base+otherRound.ID+"/"+action, host.SessionToken, nil, 404)
			e.expect("POST", base+"not-a-uuid/"+action, host.SessionToken, nil, 404)
		})
	}
	e.t = t

	var revealed struct {
		Votes []model.RevealedVote `json:"votes"`
	}
	e.expect("POST", base+round.ID+"/reveal", host.SessionToken, nil, 200).decode(t, &revealed)
	if len(revealed.Votes) != 1 || revealed.Votes[0] != (model.RevealedVote{
		ParticipantID: guest.Participant.ID, DisplayName: "Guest", Value: "8",
	}) {
		t.Errorf("revealed = %+v", revealed.Votes)
	}
	e.expect("POST", base+round.ID+"/reveal", host.SessionToken, nil, 409)

	var reset struct{ Round model.VotingRound }
	e.expect("POST", base+round.ID+"/reset", host.SessionToken, nil, 200).decode(t, &reset)
	if reset.Round.ID != round.ID || reset.Round.Status != model.RoundStatusVoting {
		t.Errorf("reset = %+v", reset.Round)
	}
	cur, _ := e.current(host.Room.Code, host.SessionToken)
	if cur.Round.Status != model.RoundStatusVoting || len(cur.Votes) != 0 {
		t.Errorf("after reset: %+v", cur)
	}

	// Other room's round is untouched by the rejected calls.
	oc, _ := e.current(other.Room.Code, "")
	if oc.Round.Status != model.RoundStatusVoting {
		t.Errorf("other room round = %+v", oc.Round)
	}
}

func TestInternalErrorsAre500(t *testing.T) {
	e := newEnv(t)
	host := e.createRoom("Room", "Host")
	e.store.Err = errors.New("db down")
	r := e.expect("GET", "/api/rooms/"+host.Room.Code, "", nil, 500)
	if strings.Contains(string(r.Body), "db down") {
		t.Errorf("internal error leaked: %s", r.Body)
	}
}

func TestWebSocketRejectsBadAuth(t *testing.T) {
	e := newEnv(t)
	a := e.createRoom("A", "HostA")
	b := e.createRoom("B", "HostB")
	base := "/api/rooms/" + a.Room.Code + "/ws"
	e.expect("GET", base, "", nil, 401)
	e.expect("GET", base+"?token=garbage", "", nil, 401)
	e.expect("GET", base+"?token="+b.SessionToken, "", nil, 403)
	e.expect("GET", "/api/rooms/ZZZZZZ/ws?token="+a.SessionToken, "", nil, 404)
}
