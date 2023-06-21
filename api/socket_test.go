package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asim/turbo/event"
	"github.com/gorilla/websocket"
)

func TestServeWebSocket(t *testing.T) {
	sub, err := event.Subscribe("test")
	if err != nil {
		t.Fatal(err)
	}

	// create a test server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocket(r) {
			serveWebSocket(w, r, sub)
		}
	}))
	defer srv.Close()

	// connect to the test server
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("could not connect to test server: %v", err)
	}
	defer conn.Close()

	// test send/receive
	msgs := []string{"hello", "world"}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for _, msg := range msgs {
			err := event.Publish("test", msg)
			if err != nil {
				t.Errorf("could not send message: %v", err)
				return
			}
		}
		cancel()
	}()

	var received []string

	var tries int
LOOP:
	for {
		select {
		case <-ctx.Done():
			if len(received) == len(msgs) || tries == 3 {
				break LOOP
			}
		default:
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("could not read message: %v", err)
			return
		}
		received = append(received, string(bytes.TrimSpace(message)))
		tries++
	}

	if len(received) != len(msgs) {
		t.Errorf("expected %d messages but received %d", len(msgs), len(received))
	}
}
