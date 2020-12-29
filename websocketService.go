package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

type WebSocketService struct {
	WebServer *httpService
	WsHub     *wsHub
}

const (
	writeWait      = 20 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

var (
	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  maxMessageSize,
		WriteBufferSize: maxMessageSize,
	}
)

// Client
type wsClient struct {
	WSHub  wsHub
	WSConn websocket.Conn
	Send   chan []byte
}

// WSHub (manage clients / broadcasts)
type wsHub struct {
	WsClients  map[*wsClient]bool
	Broadcast  chan []byte
	Register   chan *wsClient
	Unregister chan *wsClient
}

func newWSHub() *wsHub {
	return &wsHub{
		Broadcast:  make(chan []byte, 2048),
		Register:   make(chan *wsClient, 3),
		Unregister: make(chan *wsClient, 3),
		WsClients:  make(map[*wsClient]bool),
	}
}

func (w *WebSocketService) Init() error {
	logger.Printf("[Init WebSocket Service]")
	return nil
}

func websocketUpgrade(hub *wsHub, w http.ResponseWriter, r *http.Request) {
	logger.Printf("[websocketUpgrade] *** Websocket Upgrade Request from %s", r.RemoteAddr)
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[websocketUpgrade] %s Websocket Upgrade Error: %s", r.RemoteAddr, err)
		return
	}
	client := &wsClient{WSHub: *hub, WSConn: *conn, Send: make(chan []byte, maxMessageSize)}
	hub.Register <- client
	go client.writePump()
	go client.readPump()
}

// client readPump reads
func (c *wsClient) readPump() {
	defer func() {
		logger.Printf("[readPump] Unregistering: %s", c.WSConn.RemoteAddr())
		c.WSHub.Unregister <- c
		c.WSConn.Close()
	}()
	c.WSConn.SetReadLimit(maxMessageSize)
	c.WSConn.SetReadDeadline(time.Now().Add(pongWait))
	c.WSConn.SetPongHandler(func(string) error { c.WSConn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.WSConn.ReadMessage()
		logger.Printf("[ws@%s] %s", c.WSConn.RemoteAddr(), message)
		if err != nil {
			logger.Printf("[readPump] Read error1: %v", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Printf("[readPump] Read error2: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, []byte{'\n'}, []byte{' '}, -1))
		c.WSHub.Broadcast <- message
	}
}

// client writePump writes
func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		logger.Printf("[writePump] Ending WritePump: %s", c.WSConn.RemoteAddr())
		ticker.Stop()
		c.WSConn.Close()
	}()
	for {
		select {
		case message := <-c.Send:
			{
				c.WSConn.SetWriteDeadline(time.Now().Add(writeWait))
				w, err := c.WSConn.NextWriter(websocket.TextMessage)
				if err != nil {
					logger.Printf("[writePump] Write Closed on [%s]", err)
					return
				}
				b, err := w.Write(message)
				if err != nil {
					logger.Printf("[writePump] Write Failed on [%s]", err)
					return
				}
				logger.Printf("Wrote [%d bytes] to [%s]", b, c.WSConn.RemoteAddr())
				if err := w.Close(); err != nil {
					logger.Printf("[writePump] Closed on [%s]", err)
					return
				}
			}
		case <-ticker.C:
			{
				c.WSConn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.WSConn.WriteMessage(websocket.PingMessage, []byte("Ping")); err != nil {
					logger.Printf("[writePump] Closed on [%s]", err)
					return
				}

				c.WSConn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.WSConn.WriteMessage(websocket.TextMessage, []byte(".")); err != nil {
					logger.Printf("[writePump] Closed on [%s]", err)
					return
				}
			}
		}
	}
}

func (h *wsHub) run() {
	for {
		select {
		case wsc := <-h.Register:
			{
				logger.Printf("[wsHub::Register] %s", wsc.WSConn.RemoteAddr())
				h.WsClients[wsc] = true
			}
		case wsc := <-h.Unregister:
			{
				logger.Printf("[wsHub::Unregister] %s", wsc.WSConn.RemoteAddr())
				if _, ok := h.WsClients[wsc]; ok {
					delete(h.WsClients, wsc)
					close(wsc.Send)
				}
			}
		case message := <-h.Broadcast:
			{
				for wsc := range h.WsClients {
					select {
					case wsc.Send <- message:
					default:
						close(wsc.Send)
						delete(h.WsClients, wsc)
					}
				}
			}
		}
	}
}
