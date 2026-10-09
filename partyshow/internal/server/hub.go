package server

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ralpheichelberger/partyshow/internal/game"
)

const (
	writeWait  = 5 * time.Second
	pongWait   = 40 * time.Second
	pingPeriod = 25 * time.Second
	maxMsgSize = 1024
)

// conn is one WebSocket: a host screen or a player's phone.
type conn struct {
	ws       *websocket.Conn
	send     chan []byte   // control messages (joined, room, errors), in order
	wake     chan struct{} // signals that a newer state is waiting
	code     string
	host     bool
	playerID string

	mu     sync.Mutex
	closed bool
	state  []byte // latest state; older unsent states are simply replaced
}

func newConn(ws *websocket.Conn) *conn {
	return &conn{ws: ws, send: make(chan []byte, 16), wake: make(chan struct{}, 1)}
}

// queue sends a control message without blocking.
func (c *conn) queue(msg []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- msg:
	default:
	}
}

// setState replaces the pending state, so a slow phone always gets the newest one.
func (c *conn) setState(msg []byte) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.state = msg
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *conn) takeState() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := c.state
	c.state = nil
	return msg
}

// close lets the write loop flush queued messages, then closes the socket.
func (c *conn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

func (c *conn) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-c.wake:
			if msg := c.takeState(); msg != nil {
				_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// hub tracks the connections of each room and pushes fresh state to them.
type hub struct {
	mu    sync.Mutex
	conns map[string]map[*conn]bool
	rooms *game.Rooms
}

func newHub(rooms *game.Rooms) *hub {
	return &hub{conns: map[string]map[*conn]bool{}, rooms: rooms}
}

func (h *hub) add(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[c.code] == nil {
		h.conns[c.code] = map[*conn]bool{}
	}
	h.conns[c.code][c] = true
}

func (h *hub) remove(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.conns[c.code]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, c.code)
		}
	}
}

// broadcast sends every connection in the room its own view of the state.
func (h *hub) broadcast(code string) {
	room, err := h.rooms.Get(code)
	if err != nil {
		return
	}
	h.mu.Lock()
	targets := make([]*conn, 0, len(h.conns[code]))
	for c := range h.conns[code] {
		targets = append(targets, c)
	}
	h.mu.Unlock()

	var hostMsg []byte
	for _, c := range targets {
		if c.host {
			if hostMsg == nil {
				hostMsg = mustJSON(map[string]any{"type": "state", "state": room.HostView()})
			}
			c.setState(hostMsg)
			continue
		}
		if c.playerID == "" {
			continue
		}
		pv, err := room.PlayerView(c.playerID)
		if err != nil {
			c.queue(mustJSON(map[string]any{"type": "kicked"}))
			c.close()
			continue
		}
		c.setState(mustJSON(map[string]any{"type": "state", "state": pv}))
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
