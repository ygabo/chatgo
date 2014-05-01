// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type hub struct {

	// ID of hub
	HubID string `form:"-" gorethink:"id,omitempty""`

	// Name of the hub
	HubName string `form:"name" gorethink:"name"`

	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

type hubManager struct {
	hubMap     map[string]*hub            // maps hub IDs to the actual hub objects
	userHubMap map[string]map[string]bool // each user has a collection of hubs
	defaultHub *hub                       // default hub everyone connects to first
}

var h *hubManager

func init() {
	h = &hubManager{
		hubMap:     make(map[string]*hub),
		userHubMap: make(map[string]map[string]bool),
		defaultHub: newHub("default", nil),
	}
	h.hubMap[h.defaultHub.HubName] = h.defaultHub

	go h.defaultHub.run()
}

// newHub return's a new hub object
// It takes in a connection that will be inserted into the hub
func newHub(hubName string, con *connection) *hub {
	newH := &hub{
		HubName:     hubName,
		broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}

	// TODO, duplicates
	// register me in the hubmap
	// insert to DB
	if h != nil {
		h.hubMap[hubName] = newH
	}

	if con != nil {
		newH.connections[con] = true
	}
	return newH
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			// add to db
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
			// TODO close down hub if no connections
			// unless it's default hub
			// remove from db
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
