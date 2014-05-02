// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type hub struct {
	HubID    string          `form:"-" gorethink:"id,omitempty""`
	HubName  string          `form:"name" gorethink:"name"`
	HubUsers map[string]bool `form:"-" gorethink:"users"`

	connections map[*connection]bool `form:"-" gorethink:"-"`
	broadcast   chan []byte          `form:"-" gorethink:"-"`
	register    chan *connection     `form:"-" gorethink:"-"`
	unregister  chan *connection     `form:"-" gorethink:"-"`
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

	query := rethink.Table("hub").Filter(rethink.Row.Field("name").Eq("default"))
	row, err := query.RunRow(dbSession)
	if err == nil && row.IsNil() {
		fmt.Println("row nil")
		err = rethink.Table("hub").Insert(h.hubMap).RunWrite(dbSession)
	}
	if err != nil {
		fmt.Println("Default insert error,", err)
	}

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

	// register me in the hubmap
	if newH != nil {
		newH.hubMap[hubName] = newH
	}

	query := rethink.Table("hub").Filter(rethink.Row.Field("name").Eq(hubName))
	row, err := query.RunRow(dbSession)

	if err == nil && row.IsNil() {
		fmt.Println("row nil")
		err = rethink.Table("hub").Insert(newH).RunWrite(dbSession)
	}

	if err != nil {
		fmt.Println("Error newHub", err)
		return nil
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
