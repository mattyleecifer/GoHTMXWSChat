// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub
	id  uuid.UUID
	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type ChatMessage struct {
	Sender  string `json:"Sender"`
	Message string `json:"Message"`
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Parse the JSON-encoded message
		var msg map[string]interface{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			// Handle error
			continue
		}

		// Extract the chat message
		chatMessage, ok := msg["chatinput"].(string)
		if !ok {
			// Handle error
			continue
		}

		newMessage := ChatMessage{
			Sender:  c.id.String(),
			Message: chatMessage,
		}

		message, err = json.Marshal(newMessage)
		if err != nil {
			log.Println("Error marshaling JSON:", err)
			return
		}

		c.hub.broadcast <- message
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			var newMessage ChatMessage
			err = json.Unmarshal(message, &newMessage)
			if err != nil {
				log.Println("Error unmarshaling JSON:", err)
				return
			}

			fmt.Println(newMessage)
			if newMessage.Sender == c.id.String() {
				// if its to self
				if newMessage.Message == "{{typing}}" {
					message = []byte(string(``))
				} else {
					message = []byte(string(`<div id="chatloading" hx-swap-oob="beforebegin"><p><strong>You</strong>: `) + string(newMessage.Message) + string(`</p><div hx-get="/scroll" hx-target="#chat_room" hx-swap="beforebegin scroll:#chat_room:bottom" hx-trigger="load"></div></div><input id="chatinput" name="chatinput" autocomplete="off" autofocus hx-select-oob="#chatinput" hx-swap="none scroll:#chat_room:bottom"><div id="chatloading" class="htmx-indicator" hx-swap-oob="outerHTML"><p>Someone is typing...</p></div>`))
				}
			} else {
				if newMessage.Message == "{{typing}}" {
					message = []byte(string(`<div id="chatloading" hx-swap-oob="beforebegin"><div hx-trigger="load" hx-get="/sleep" hx-target="#chatloading" hx-indicator="#chatloading" hx-swap="beforebegin"></div><div hx-get="/scroll" hx-target="#chat_room" hx-swap="beforebegin scroll:#chat_room:bottom" hx-trigger="load"></div></div>`))
				} else {
					message = []byte(string(`<div id="chatloading" hx-swap-oob="beforebegin"><p><strong>`) + c.id.String() + string("</strong>: ") + string(newMessage.Message) + string(`</p><div hx-get="/scroll" hx-target="#chat_room" hx-swap="beforebegin scroll:#chat_room:bottom" hx-trigger="load"></div></div><input id="chatinput" name="chatinput" autocomplete="off" autofocus hx-select-oob="#chatinput" hx-swap="none scroll:#chat_room:bottom"><div id="chatloading" class="htmx-indicator" hx-swap-oob="outerHTML"><p>Someone is typing...</p></div>`))
				}
			}

			// add wrapper to send message to htmx
			message = []byte(string(message))

			fmt.Println(string(message))
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}
	client := &Client{hub: hub, id: id, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
