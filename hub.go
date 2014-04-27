// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

var hubMap map[string]*hub                // maps hub IDs to the actual hub objects
var userHubMap map[string]map[string]bool // each user has a collection of hubs
var h *hub                                // h to initialize the default hub everyone connects to first

func init() {
	hubMap = make(map[string]*hub)
	userHubMap = make(map[string]map[string]bool)
	h = newHub(nil)
	hubMap["default"] = h

	go h.run()
}

// newHub return's a new hub object
// It takes in a connection that will be inserted into the hub
func newHub(con *connection) *hub {
	h := hub{
		broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
	if con != nil {
		h.connections[con] = true
	}
	return &h
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
			// TODO close down hub if no connections
			// unless it's default hub
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					close(c.send)
					delete(h.connections, c)
				}
			}
		}
	}
}
