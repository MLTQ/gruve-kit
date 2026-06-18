# gruve-sdk (Python)

Make your Python app discoverable in your friends' Gruve lobbies. One call:

```python
import gruve_sdk

gruve_sdk.announce(id="pharaoh", name="Pharaoh", port=18000, hue=45,
                   upstreams={"tts": 18001, "sfx": 18002})
```

While your app runs it appears as a tile on every member's mesh; they click it and your app
opens for them, served from your machine over the mesh. When it exits (or crashes) the tile
disappears within the TTL.

**Conformance:** Gruve Adapter Protocol **levels 1 (announce) + 2 (dispatch, backend side)**.
Shared-state sessions (L3) run in the browser, so they live in the JavaScript SDK — a Python
backend never needs them. **Zero dependencies** (standard library only).

## What this package is (and is NOT)

The Gruve **agent** on your machine owns the mesh node, identity, discovery, and proxying. This
package is just the **announce protocol**: a heartbeat POST to the agent on `127.0.0.1:8088`
saying "I'm running on port X," plus a helper to reach mesh capabilities by name. Your app does
no networking beyond that — which is the point.

It's designed for the **long-lived backend process** — a FastAPI/Flask server, an ML inference
service, a data pipeline. That's the thing that should announce (never a UI page, whose timers get
throttled and whose registration expires mid-session).

## Announce

```python
import gruve_sdk

# start your HTTP server FIRST — the agent probes the port before listing you (behavior 1.4)
gruve = gruve_sdk.announce(
    id="hunger",            # ^[a-z0-9][a-z0-9-]{0,31}$
    name="Hunger",          # lobby display name
    port=9700,              # the localhost port your UI/HTTP surface is on
    blurb="novelty-driven crawler",
    hue=95,                 # tile hue 0–360 (optional)
    upstreams={"api": 9701},# named backend ports, reached from the frontend via apiBase("api")
)
# ... runs for the life of your process ...
gruve.stop()                # withdraw early (also happens automatically at interpreter exit)
```

`announce()` heartbeats from a daemon thread and **never blocks or crashes your app**: if the agent
isn't running it retries quietly, and your tile appears the moment the agent starts. Your app works
identically with no Gruve installed.

### A headless capability (no lobby tile)

Set `service=True` to publish a capability at `/svc/<id>/` that any friend's app can consume by
name — often the highest-value integration for the least work (a model server, a parser, a feed):

```python
gruve_sdk.announce(id="inference", port=8000, service=True)
```

## Dispatch — reach a mesh capability by name (L2)

When *your* backend needs another node's capability, ask for it by name instead of an address:

```python
import urllib.request
from gruve_sdk import service_base

LLM = service_base("inference")          # -> http://127.0.0.1:8088/svc/inference
urllib.request.urlopen(LLM + "/v1/chat/completions", data=...)
```

The local agent resolves a provider (local first, then any joined network); your code never knows
which machine answers.

> Note: `apiBase()` — the *frontend* helper that rewrites your own backend URL when a page is served
> through the mesh — is a browser concern and lives in the JavaScript SDK. A Python backend doesn't
> need it.

## Install

Until it's on PyPI, install from the kit:

```bash
pip install ./sdk-py        # or:  pip install -e ./sdk-py  for local dev
```

## Try it

With a local agent running (`./gruve`, no network needed):

```bash
python examples/announce_demo.py    # serves a page on :9702 and announces "Python Demo"
```

Open the lobby — a *Python Demo* tile appears under "Your machine." Ctrl-C withdraws it.

## The whole protocol, if you don't even want the package

```bash
curl -X POST http://127.0.0.1:8088/gruve/announce \
  -H 'content-type: application/json' \
  -d '{"id":"myapp","name":"My App","port":9000,"hue":120,"ttl":60}'
# re-POST every ttl/3 seconds; DELETE /gruve/announce?id=myapp when done
```

See `PROTOCOL.md` for the normative contract.
