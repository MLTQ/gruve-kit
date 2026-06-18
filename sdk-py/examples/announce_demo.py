"""Conformance demo: a tiny stdlib HTTP server, announced to the local agent.

Run a local agent (`./gruve`), then:  python examples/announce_demo.py
(from the sdk-py/ directory, or after `pip install -e .`)
"""
from http.server import BaseHTTPRequestHandler, HTTPServer

import gruve_sdk


class Hello(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("content-type", "text/html")
        self.end_headers()
        self.wfile.write(
            b"<body style='background:#141821;color:#7aa2f7;font-family:monospace;"
            b"display:grid;place-items:center;height:100vh'>"
            b"<h1>hello from python \xf0\x9f\x90\x8d</h1></body>"
        )

    def log_message(self, *_):  # keep the console quiet
        pass


# 1.4: listen BEFORE announcing — the agent probes the port.
srv = HTTPServer(("127.0.0.1", 9702), Hello)
print("python demo app on :9702")

gruve = gruve_sdk.announce(
    id="pydemo", name="Python Demo", port=9702,
    blurb="announced by gruve-sdk (python)", hue=210,
)
print("announced; check the lobby. ctrl-c to exit (withdraws).")

try:
    srv.serve_forever()
except KeyboardInterrupt:
    gruve.stop()
