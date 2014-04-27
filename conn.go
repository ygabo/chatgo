// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/gorilla/websocket"
	"github.com/martini-contrib/sessionauth"
	"net/http"
	"time"
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// connection is an middleman between the websocket connection and the hub.
type connection struct {
	// user associated with this connection
	userID string

	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type messageFrom struct {
	hubID string `json:"hub_id"`
	body  string `json:"body"`
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
		for userHub := range userHubMap[c.userID] {
			hubMap[userHub].unregister <- c
		}
		delete(userHubMap, c.userID)
		delete(connMap, c.userID)
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		msg := messageFrom{}
		err := c.ws.ReadJSON(msg)
		if err != nil {
			break
		}
		// Send the message to the proper hub
		// Check if user has is part of the hub first.
		// Then send the message to the hub.
		if userHubMa[c.userID][msg.hubID] {
			hubMap[msg.hubID].broadcast <- msg
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
			if err := c.write(websocket.TextMessage, message); err != nil {
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
	userID := user.UniqueId().(string)
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		return
	} else if err != nil {
		return
	}

	c := &connection{userID: userID, send: make(chan []byte, 256), ws: ws}

	connMap[userID] = c                  // remember user's connection
	userHubMap[userID]["default"] = true // add default hub into user's hubs
	hubMap["default"].register <- c      // register the user in the default hub
	go c.writePump()
	c.readPump()
}
