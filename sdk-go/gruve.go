// Package gruve makes a Go app discoverable on a Gruve mesh.
//
// It implements the Gruve Adapter Protocol, levels 1 (announce) and 2 (dispatch). The agent
// lives on localhost, so the standard library is all we need — zero dependencies.
//
//	// after your HTTP server is LISTENING (the agent probes the port):
//	h, _ := gruve.Announce(gruve.Options{
//		ID: "hunger", Name: "Hunger", Port: 9700,
//		Blurb: "novelty-driven crawler", Hue: 95,
//		Upstreams: map[string]int{"api": 9701},
//	})
//	defer h.Stop() // withdraw on clean shutdown; TTL reaps it on a crash
//
// A headless capability instead of a lobby app (/svc/<id>/, no tile):
//
//	gruve.Announce(gruve.Options{ID: "inference", Port: 8000, Service: true})
//	llm := gruve.ServiceBase("inference") // consume mesh capabilities by name
package gruve

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Version of the SDK.
const Version = "0.1.0"

const defaultAgent = "127.0.0.1:8088"

var client = &http.Client{Timeout: 2 * time.Second}

// AgentAddr returns the agent address as host:port. Override with the GRUVE_AGENT env var.
func AgentAddr() string {
	addr := strings.TrimSpace(os.Getenv("GRUVE_AGENT"))
	if addr == "" {
		return defaultAgent
	}
	if i := strings.Index(addr, "://"); i >= 0 { // tolerate a full URL
		addr = addr[i+3:]
	}
	return strings.TrimRight(addr, "/")
}

func agentURL(path string) string { return "http://" + AgentAddr() + path }

// ServiceBase returns the base URL for a named mesh capability, resolved by the local agent
// (protocol L2). ServiceBase("inference") -> "http://127.0.0.1:8088/svc/inference". The agent
// picks a provider (local first, then any joined network); the caller never knows where it lives.
func ServiceBase(name string) string { return agentURL("/svc/" + name) }

// Options configures an announcement (protocol L1).
type Options struct {
	ID        string         // required; ^[a-z0-9][a-z0-9-]{0,31}$
	Name      string         // lobby display name; defaults to ID for services
	Port      int            // required; localhost port your HTTP surface is on
	TTL       int            // seconds until the agent forgets us; clamped 5..300 (default 60)
	Hue       int            // tile hue 0-360 (default 250)
	Blurb     string         // optional one-liner
	Icon      string         // optional lobby glyph
	Upstreams map[string]int // optional named backend ports, reached from the frontend via apiBase(name)
	Service   bool           // true = mesh capability at /svc/<id>/, no lobby tile
}

// Handle controls a running announcement.
type Handle struct {
	once sync.Once
	done chan struct{}
	dead chan struct{} // closed once the withdraw has been sent
}

// Stop withdraws the announcement and ends the heartbeat, blocking briefly until the
// withdraw is sent. Safe to call more than once.
func (h *Handle) Stop() {
	h.once.Do(func() { close(h.done) })
	<-h.dead
}

// Announce registers a running app (or service) with the local Gruve agent (protocol L1).
//
// It heartbeats every TTL/3 seconds from a background goroutine and never blocks or crashes the
// host app: if the agent isn't running it retries quietly, and the tile appears the moment it
// starts. Returns a Handle; call Stop (defer it) to withdraw on clean shutdown. The only error is
// validation — missing ID, Port, or Name (for non-services).
func Announce(opts Options) (*Handle, error) {
	if opts.Service && opts.Name == "" {
		opts.Name = opts.ID
	}
	if opts.ID == "" || opts.Name == "" || opts.Port == 0 {
		return nil, errors.New("gruve: Announce requires ID, Name and Port")
	}

	ttl := opts.TTL
	if ttl == 0 {
		ttl = 60
	}
	if ttl < 5 {
		ttl = 5
	} else if ttl > 300 {
		ttl = 300
	}
	hue := opts.Hue
	if hue == 0 {
		hue = 250
	}
	ups := opts.Upstreams
	if ups == nil {
		ups = map[string]int{}
	}
	payload := map[string]any{
		"id": opts.ID, "name": opts.Name, "port": opts.Port, "ttl": ttl,
		"hue": hue, "blurb": opts.Blurb, "upstreams": ups, "service": opts.Service,
	}
	if opts.Icon != "" {
		payload["icon"] = opts.Icon
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	beat := time.Duration(ttl/3) * time.Second
	if beat < 2*time.Second {
		beat = 2 * time.Second
	}
	h := &Handle{done: make(chan struct{}), dead: make(chan struct{})}

	go func() {
		defer close(h.dead)
		request(http.MethodPost, "/gruve/announce", body) // first beat immediately (1.1)
		t := time.NewTicker(beat)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				request(http.MethodPost, "/gruve/announce", body)
			case <-h.done:
				q := "?id=" + url.QueryEscape(opts.ID)
				if opts.Service {
					q += "&service=1"
				}
				request(http.MethodDelete, "/gruve/announce"+q, nil) // best-effort withdraw (1.5)
				return
			}
		}
	}()
	return h, nil
}

// request makes a minimal HTTP call to the local agent; failures are swallowed (1.2 — the agent
// being absent is expected, and the host app must work identically with no Gruve installed).
func request(method, path string, body []byte) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, agentURL(path), r)
	if err != nil {
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
