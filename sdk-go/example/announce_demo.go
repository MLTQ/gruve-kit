// Conformance demo: a tiny net/http server, announced to the local agent.
// Run a local agent (./gruve), then, from sdk-go/:  go run ./example
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	gruve "github.com/MLTQ/gruve-kit/sdk-go"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<body style='background:#141821;color:#8ce0b0;font-family:monospace;"+
			"display:grid;place-items:center;height:100vh'><h1>hello from go \U0001f439</h1></body>")
	})

	// 1.4: listen BEFORE announcing — the agent probes the port.
	ln, err := net.Listen("tcp", "127.0.0.1:9703")
	if err != nil {
		panic(err)
	}
	fmt.Println("go demo app on :9703")
	go http.Serve(ln, mux)

	h, err := gruve.Announce(gruve.Options{
		ID: "godemo", Name: "Go Demo", Port: 9703,
		Blurb: "announced by gruve-sdk (go)", Hue: 140,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("announced; check the lobby. ctrl-c to exit (withdraws).")

	// Signal handling belongs to the app, not the SDK: withdraw on Ctrl-C.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	h.Stop()
}
