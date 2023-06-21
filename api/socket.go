package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asim/turbo/event"
	"github.com/asim/turbo/log"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = 15 * time.Second

	// Maximum message size allowed from client.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// check if the request is for websockets
func isWebSocket(r *http.Request) bool {
	contains := func(key, val string) bool {
		vv := strings.Split(r.Header.Get(key), ",")
		for _, v := range vv {
			if val == strings.ToLower(strings.TrimSpace(v)) {
				return true
			}
		}
		return false
	}

	if contains("Connection", "upgrade") && contains("Upgrade", "websocket") {
		return true
	}

	return false
}

// serve an actual websocket
func serveWebSocket(w http.ResponseWriter, r *http.Request, sub *event.Subscriber) {
	var rspHdr http.Header
	// we use Sec-Websocket-Protocol to pass auth headers so just accept anything here
	if prots := r.Header.Values("Sec-WebSocket-Protocol"); len(prots) > 0 {
		rspHdr = http.Header{}
		for _, p := range prots {
			rspHdr.Add("Sec-WebSocket-Protocol", p)
		}
	}

	// upgrade the connection
	conn, err := upgrader.Upgrade(w, r, rspHdr)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// assume text types
	msgType := websocket.TextMessage

	// create a stream
	s := stream{
		ctx:         r.Context(),
		conn:        conn,
		sub:         sub,
		messageType: msgType,
	}

	// start processing the stream
	s.run()
}

type stream struct {
	// message type requested (binary or text)
	messageType int
	// request context
	ctx context.Context
	// the websocket connection.
	conn *websocket.Conn
	// the downstream connection.
	sub *event.Subscriber
}

func (s *stream) run() {
	defer func() {
		s.conn.Close()
	}()

	// for our messages from sub
	msgs := make(chan interface{}, 100)

	// to cancel everything
	stopCtx, cancel := context.WithCancel(context.Background())

	// wait for things to exist
	wg := sync.WaitGroup{}
	wg.Add(3)

	// establish the loops
	go s.rspToBufLoop(cancel, &wg, stopCtx, msgs)
	go s.bufToClientLoop(cancel, &wg, stopCtx, msgs)
	go s.clientToServerLoop(cancel, &wg, stopCtx)
	wg.Wait()
}

func (s *stream) clientToServerLoop(cancel context.CancelFunc, wg *sync.WaitGroup, stopCtx context.Context) {
	defer func() {
		s.sub.Close()
		cancel()
		wg.Done()
	}()
	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error { s.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		select {
		case <-stopCtx.Done():
			return
		default:
		}

		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Print(err)
			}
			return
		}

		// TODO: pass on client to server messages
		log.Print("Got msg", msg)
	}

}

func (s *stream) rspToBufLoop(cancel context.CancelFunc, wg *sync.WaitGroup, stopCtx context.Context, msgs chan interface{}) {
	defer func() {
		cancel()
		wg.Done()
	}()

	for {
		var msg json.RawMessage
		if err := s.sub.Next(stopCtx, &msg); err == io.EOF {
			return
		} else if err != nil {
			b, _ := json.Marshal(err)
			s.conn.WriteMessage(s.messageType, b)
			s.conn.WriteMessage(websocket.CloseAbnormalClosure, []byte{})
			return
		}

		select {
		case msgs <- &msg:
		case <-stopCtx.Done():
		default:
		}
	}

}

func (s *stream) bufToClientLoop(cancel context.CancelFunc, wg *sync.WaitGroup, stopCtx context.Context, msgs chan interface{}) {
	defer func() {
		s.conn.Close()
		cancel()
		wg.Done()

	}()
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stopCtx.Done():
			return
		case <-s.ctx.Done():
			return
		case <-s.sub.Exit:
			s.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case msg := <-msgs:
			// read response body
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			w, err := s.conn.NextWriter(s.messageType)
			if err != nil {
				return
			}
			b, _ := json.Marshal(msg)
			if _, err := w.Write(b); err != nil {
				return
			}
			if err := w.Close(); err != nil {
				return
			}
		}
	}

}
