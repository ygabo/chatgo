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
	"github.com/martini-contrib/render"
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
	send chan []byte
}

type messageFrom struct {
	HubID string `json:"hub_id"`
	Body  string `json:"body"`
	From  string `json:"from,omitempty"`
}

// connMap maps the userIDs to the websocket connection
var connMap map[string]*connection

func init() {
	connMap = make(map[string]*connection)
}

func getHub(r render.Render) {
	r.HTML(200, "room", nil)
}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {
	defer func() {
		// if this conn is closed, user is done
		// unregister from all its hubs, clean the maps
		h.remEdge <- hubConnMsg{Con: con}
		delete(connMap, c.userID)
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	fmt.Println("Started read pump:", c.userID)
	for {
		msg := messageFrom{}
		err := c.ws.ReadJSON(&msg)
		msg.From = c.userName

		if err != nil {
			fmt.Println("msg error: ", err)
			break
		}

		// Send the message to the proper hub
		// Check if user is part of the hub first.
		// Then send the message to the hub.
		b, err := json.Marshal(msg)
		if err != nil {
			h.bCastToHub <- hubConnMsg{Con: C, HubID: msg.HubID, Msg: b}
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
	currUser := user.(*User)
	userID := currUser.Id
	userName := currUser.Username

	if userDuplicate := connMap[userID]; userDuplicate != nil {
		return // user already has websocket connection
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		fmt.Println("Error with handshake, not ok. ", ok)
		return
	} else if err != nil {
		fmt.Println("Handshake error, ", err)
		return
	}

	c := &connection{
		userID:   userID,
		userName: userName,
		send:     make(chan []byte, 256),
		ws:       ws,
	}
	connMap[userID] = c // remember user's connection

	if h.defaultHub == nil {
		h.defaultHub = newHub("default", c)
		go h.defaultHub.run()
	} else {
		h.defaultHub.register <- c
	}

	h.insertEdge(currUser, h.defaultHub)

	go c.writePump()
	c.readPump()
}
