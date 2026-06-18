package gruve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAnnounceHeartbeatAndWithdraw(t *testing.T) {
	var mu sync.Mutex
	var posts [][]byte
	var deletes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			posts = append(posts, b)
			w.Write([]byte(`{"ok":true,"ttl":6}`))
		case http.MethodDelete:
			deletes = append(deletes, r.URL.RawQuery)
		}
	}))
	defer srv.Close()
	t.Setenv("GRUVE_AGENT", strings.TrimPrefix(srv.URL, "http://"))

	h, err := Announce(Options{
		ID: "hunger", Name: "Hunger", Port: 9700,
		Hue: 95, Upstreams: map[string]int{"api": 9701}, TTL: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // first beat fires immediately
	h.Stop()                           // blocks until withdraw is sent

	mu.Lock()
	defer mu.Unlock()
	if len(posts) == 0 {
		t.Fatal("no announce POST observed")
	}
	var body map[string]any
	if err := json.Unmarshal(posts[0], &body); err != nil {
		t.Fatalf("bad JSON body: %v", err)
	}
	if body["id"] != "hunger" || body["port"].(float64) != 9700 {
		t.Fatalf("unexpected body: %v", body)
	}
	if body["service"] != false {
		t.Fatalf("service should be false: %v", body)
	}
	if body["ttl"].(float64) != 6 {
		t.Fatalf("ttl not honored: %v", body["ttl"])
	}
	if len(deletes) == 0 || !strings.Contains(deletes[0], "id=hunger") {
		t.Fatalf("withdraw not sent correctly: %v", deletes)
	}
}

func TestValidation(t *testing.T) {
	if _, err := Announce(Options{ID: "", Port: 1}); err == nil {
		t.Fatal("expected a validation error for missing fields")
	}
}

func TestServiceNameDefaultsToID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	t.Setenv("GRUVE_AGENT", strings.TrimPrefix(srv.URL, "http://"))
	h, err := Announce(Options{ID: "feed", Port: 9800, Service: true}) // no Name
	if err != nil {
		t.Fatalf("service announce should default Name to ID: %v", err)
	}
	h.Stop()
}

func TestServiceBase(t *testing.T) {
	t.Setenv("GRUVE_AGENT", "127.0.0.1:8088")
	if got := ServiceBase("inference"); got != "http://127.0.0.1:8088/svc/inference" {
		t.Fatalf("ServiceBase = %s", got)
	}
}

func TestAgentAddrToleratesURL(t *testing.T) {
	t.Setenv("GRUVE_AGENT", "http://10.0.0.5:9999/")
	if got := AgentAddr(); got != "10.0.0.5:9999" {
		t.Fatalf("AgentAddr = %s", got)
	}
}
