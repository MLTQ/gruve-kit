"""gruve-sdk — make a Python app discoverable on a Gruve mesh.

Implements the Gruve Adapter Protocol, levels 1 (announce) and 2 (dispatch, backend
side). Zero dependencies: the agent lives on localhost, so the standard library is all
we need.

    # after your HTTP server is LISTENING (the agent probes the port):
    import gruve_sdk
    gruve = gruve_sdk.announce(id="hunger", name="Hunger", port=9700,
                               blurb="novelty-driven crawler", hue=95,
                               upstreams={"api": 9701})
    # heartbeats from a daemon thread; agent absent = silently retried (your app works
    # identically without Gruve). Withdraws on gruve.stop() or interpreter exit.

A capability instead of a lobby app (/svc/<id>/, no tile):

    gruve_sdk.announce(id="osint-feed", port=9800, service=True)
    llm = gruve_sdk.service_base("inference")  # consume mesh capabilities by name
"""
from __future__ import annotations

import atexit
import json
import os
import threading
import urllib.error
import urllib.parse
import urllib.request
from typing import Dict, Optional

__all__ = ["announce", "service_base", "agent_addr", "AnnounceHandle"]
__version__ = "0.1.0"

_TIMEOUT = 2.0  # seconds; the agent is on localhost, so this is generous


def agent_addr() -> str:
    """Agent address as ``host:port``. Override with the ``GRUVE_AGENT`` env var."""
    addr = os.environ.get("GRUVE_AGENT", "127.0.0.1:8088").strip()
    if "://" in addr:  # tolerate a full URL in the env var
        addr = addr.split("://", 1)[1]
    return addr.rstrip("/")


def _agent_url(path: str) -> str:
    return "http://{}{}".format(agent_addr(), path)


def service_base(name: str) -> str:
    """Base URL for a named mesh capability, resolved by the local agent (protocol L2).

    ``service_base("inference")`` -> ``http://127.0.0.1:8088/svc/inference``. The agent
    picks a provider (local first, then any joined network); the caller never knows where
    it lives. Use it where you build URLs for a backend dependency::

        LLM = service_base("inference")
        urllib.request.urlopen(LLM + "/v1/chat/completions", ...)
    """
    return _agent_url("/svc/{}".format(name))


def _request(method: str, path: str, body: Optional[bytes] = None) -> int:
    """Minimal HTTP call to the local agent; returns the status code (0 on failure)."""
    req = urllib.request.Request(_agent_url(path), data=body, method=method)
    if body is not None:
        req.add_header("content-type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=_TIMEOUT) as r:
            return r.status
    except urllib.error.HTTPError as e:
        return e.code
    except Exception:
        return 0  # agent absent/unreachable — expected (1.2); the caller stays quiet


class AnnounceHandle:
    """Controls a running announcement.

    ``stop()`` withdraws it and ends the heartbeat; it also runs automatically at
    interpreter exit. Idempotent.
    """

    def __init__(self, body: bytes, app_id: str, service: bool, beat_every: float):
        self._body = body
        self._id = app_id
        self._service = service
        self._beat_every = beat_every
        self._stop = threading.Event()
        # daemon thread (1.3): never keeps the host process alive or blocks it
        self._thread = threading.Thread(target=self._run, name="gruve-announce", daemon=True)
        self._thread.start()
        atexit.register(self.stop)

    def _run(self) -> None:
        while not self._stop.is_set():
            _request("POST", "/gruve/announce", self._body)
            # wait responsively so stop() withdraws promptly (1.5)
            if self._stop.wait(self._beat_every):
                break
        q = "?id=" + urllib.parse.quote(self._id) + ("&service=1" if self._service else "")
        _request("DELETE", "/gruve/announce" + q)

    def stop(self) -> None:
        """Withdraw the announcement and stop heartbeating. Idempotent."""
        self._stop.set()


def announce(
    *,
    id: str,
    port: int,
    name: str = "",
    ttl: int = 60,
    hue: int = 250,
    blurb: str = "",
    icon: str = "",
    upstreams: Optional[Dict[str, int]] = None,
    service: bool = False,
) -> AnnounceHandle:
    """Announce a running app (or service) to the local Gruve agent (protocol L1).

    Required: ``id`` (matching ``^[a-z0-9][a-z0-9-]{0,31}$``) and ``port`` (the localhost
    port your HTTP surface is already LISTENING on — the agent probes it). ``name``
    defaults to ``id`` for services. ``upstreams`` maps a name to a backend port, reachable
    from your frontend via ``apiBase(name)``. Set ``service=True`` for a headless mesh
    capability at ``/svc/<id>/`` with no lobby tile.

    Returns an :class:`AnnounceHandle`; call ``.stop()`` to withdraw early. Heartbeats every
    ``ttl/3`` seconds from a daemon thread, and never blocks or crashes the host app: if the
    agent isn't running it retries quietly and the tile appears the moment it starts.
    """
    if service and not name:
        name = id
    if not id or not name or not port:
        raise ValueError("gruve announce: id, name and port are required")
    ttl = max(5, min(300, int(ttl)))
    payload = {
        "id": id,
        "name": name,
        "port": int(port),
        "ttl": ttl,
        "hue": int(hue),
        "blurb": blurb,
        "upstreams": upstreams or {},
        "service": bool(service),
    }
    if icon:
        payload["icon"] = icon
    body = json.dumps(payload).encode("utf-8")
    beat_every = max(2.0, ttl / 3.0)
    return AnnounceHandle(body, id, bool(service), beat_every)
