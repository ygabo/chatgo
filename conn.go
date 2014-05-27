// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is edited version of the gorilla websocket example.
// This supports multiple hubs, ie multiple chatrooms.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/martini-contrib/sessionauth"
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

	// We should have a system to determine what type of message we got
	// and do actions accordingly.
	// eg.
	// 100 = normal broadcast to hubid attached
	// 200 = create room, with room name
	// 201 = rename room, must have hubid attached, must be admin
	// 300 = leave room
	// 301 = leave all
	msgTypeBroadcast  = 100
	msgTypeCreateRoom = 200
	msgTypeJoinRoom   = 201
	msgTypeLeaveRoom  = 300
	msgTypeLeaveAll   = 301
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// connection is an middleman between the websocket connection and the hub.
type connection struct {
	userID   string
	userName string

	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan msg
}

type msg struct {
	Type  int    `json:"msg_type"`
	HubID string `json:"hub_id"`
	From  string `json:"from,omitempty"`
	Body  string `json:"body"`
}

// connMap maps the userIDs to the websocket connection
var connMap map[string]*connection

func init() {
	connMap = make(map[string]*connection)
}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {
	defer func() {
		// if this conn is closed, user is done
		// unregister from all its hubs, clean the maps
		h.remEdge <- hubConnMsg{Con: c}
		delete(connMap, c.userID)
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	fmt.Println("Started read pump:", c.userID)
	for {
		msg := msg{}
		err := c.ws.ReadJSON(&msg)
		msg.From = c.userName

		if err != nil {
			fmt.Println("msg error: ", err)
			break
		}

		// Send the message to the proper hub
		// Check if user is part of the hub first.
		// Then send the message to the hub.
		fmt.Println(msg)

		if err == nil {
			if msg.Type == msgTypeBroadcast {
				h.bCastToHub <- hubConnMsg{Con: c, HubID: msg.HubID, Msg: &msg}
			} else if msg.Type == msgTypeJoinRoom {
				hub := h.HubMap[msg.HubID]
				h.addEdge <- hubConnMsg{Con: c, Hub: hub}
				// Todo reply with all users in hub and other hub metadata
			} else if msg.Type == msgTypeCreateRoom {
				// Todo
			} else if msg.Type == msgTypeLeaveRoom {
				// Todo
			} else if msg.Type == msgTypeLeaveAll {
				// Todo
			} else {
				// Todo
			}
		} else {
			fmt.Println("Error decoding message, ", err)
		}
	}
}

// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
// this doesn't care about hubIDs and let frontend handle displaying
// the message in the proper hub. (hub_id is part of the message sent to FE)
func (c *connection) writePump() {
	fmt.Println("Started write pump:", c.userID)
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			b, jsonErr := json.Marshal(message)
			if jsonErr != nil {
				fmt.Println("json marshal error", message)
				return
			}
			if err := c.write(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// wsHandler - takes care of incomming chat connection requests
// The user has to be logged in to get to this point
func wsHandler(w http.ResponseWriter, user sessionauth.User, r *http.Request) {
	currUser := user.(*User)
	userID := currUser.Id
	userName := currUser.Username

	fmt.Println("handler start", r.RemoteAddr)
	if connMap[userID] != nil {
		fmt.Println("Error user already has websocket connection ")
		return // user already has websocket connection
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		fmt.Println("Error with handshake, not ok. ", ok)
		return
	} else if err != nil {
		fmt.Println("Handshake error, ", err)
		return
	} else {
		fmt.Println("handshake ok ", userID)
	}

	c := &connection{
		userID:   userID,
		userName: userName,
		send:     make(chan msg, 64),
		ws:       ws,
	}
	connMap[userID] = c // remember user's connection

	if h.DefaultHub == nil {
		h.DefaultHub, _ = newHub("default", nil)
	}

	h.addEdge <- hubConnMsg{Con: c, Hub: h.DefaultHub}

	go c.writePump()
	c.readPump()
}
