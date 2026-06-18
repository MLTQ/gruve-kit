# gruve-sdk (Go)

Make your Go app or service discoverable in your friends' Gruve lobbies. One call:

```go
import gruve "github.com/MLTQ/gruve-kit/sdk-go"

h, _ := gruve.Announce(gruve.Options{
    ID: "doten", Name: "Doten", Port: 1420, Hue: 200,
    Upstreams: map[string]int{"api": 3030},
})
defer h.Stop()
```

While your app runs it appears as a tile on every member's mesh; they click it and it opens for
them, served from your machine. When it exits (or crashes) the tile disappears within the TTL.

**Conformance:** Gruve Adapter Protocol **levels 1 (announce) + 2 (dispatch, backend side)**.
Shared-state sessions (L3) run in the browser, so they live in the JavaScript SDK — a Go backend
never needs them. **Zero dependencies** (standard library only).

## What this package is (and is NOT)

The Gruve **agent** on your machine owns the mesh node, identity, discovery, and proxying. This
package is just the **announce protocol**: a heartbeat POST to the agent on `127.0.0.1:8088` saying
"I'm running on port X," plus a helper to reach mesh capabilities by name. Your app does no
networking beyond that.

It's a natural fit for the **headless `service: true` shape** — a homelab daemon, a model server, a
data pipeline that any friend's app can consume by name with no UI of its own.

## Announce

```go
package main

import (
    "net"
    "net/http"

    gruve "github.com/MLTQ/gruve-kit/sdk-go"
)

func main() {
    // start your HTTP server FIRST — the agent probes the port before listing you (behavior 1.4)
    ln, _ := net.Listen("tcp", "127.0.0.1:9700")
    go http.Serve(ln, handler())

    h, err := gruve.Announce(gruve.Options{
        ID:        "hunger",                 // ^[a-z0-9][a-z0-9-]{0,31}$
        Name:      "Hunger",                 // lobby display name
        Port:      9700,                      // the localhost port your UI/HTTP surface is on
        Blurb:     "novelty-driven crawler",
        Hue:       95,                        // tile hue 0–360 (optional)
        Upstreams: map[string]int{"api": 9701}, // reached from the frontend via apiBase("api")
    })
    if err != nil {
        panic(err) // the only error is a missing ID/Name/Port
    }
    defer h.Stop() // withdraw on clean shutdown; TTL reaps it on a crash

    select {} // run forever
}
```

`Announce` heartbeats from a background goroutine and **never blocks or crashes your app**: if the
agent isn't running it retries quietly, and your tile appears the moment the agent starts. Your app
works identically with no Gruve installed.

`Stop()` is best-effort and belongs on a `defer` (or your own signal handler) — on a hard crash the
agent forgets you after the TTL.

### A headless capability (no lobby tile)

```go
gruve.Announce(gruve.Options{ID: "inference", Port: 8000, Service: true})
```

Publishes a capability at `/svc/inference/` that any friend's app can consume by name — often the
highest-value integration for the least work.

## Dispatch — reach a mesh capability by name (L2)

```go
import (
    "net/http"
    gruve "github.com/MLTQ/gruve-kit/sdk-go"
)

llm := gruve.ServiceBase("inference") // -> http://127.0.0.1:8088/svc/inference
http.Post(llm+"/v1/chat/completions", "application/json", body)
```

The local agent resolves a provider (local first, then any joined network); your code never knows
which machine answers.

> Note: `apiBase()` — the *frontend* helper that rewrites your own backend URL when a page is served
> through the mesh — is a browser concern and lives in the JavaScript SDK. A Go backend doesn't need
> it.

## Install

```bash
go get github.com/MLTQ/gruve-kit/sdk-go
```

## Try it

With a local agent running (`./gruve`, no network needed), from `sdk-go/`:

```bash
go run ./example     # serves a page on :9703 and announces "Go Demo"
```

Open the lobby — a *Go Demo* tile appears under "Your machine." Ctrl-C withdraws it.

See `PROTOCOL.md` for the normative contract.
