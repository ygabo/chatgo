// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"

	r "github.com/dancannon/gorethink"
)

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type hub struct {
	HubID    string            `form:"-" gorethink:"id,omitempty""`
	HubName  string            `form:"name" gorethink:"name"`
	HubUsers map[string]string `form:"-" gorethink:"users"`

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

	query := r.Table("hub").Filter(r.Row.Field("name").Eq("default"))
	row, err := query.RunRow(dbSession)
	if row.IsNil() {
		fmt.Println("row nil")
		_, err = r.Table("hub").Insert(h.defaultHub).RunWrite(dbSession)
	} else {
		var hubInDB hub
		if scanErr := row.Scan(&hubInDB); scanErr != nil {
			fmt.Println("Scan Error")
		} else {
			h.defaultHub.HubID = hubInDB.HubID
		}
	}

	if err != nil {
		fmt.Println("Default insert error,", err, " -- Still running the default hub.")
	}

	go h.defaultHub.run()
}

// newHub return's a new hub object
// It takes in a connection that will be inserted into the hub
func newHub(hubName string, con *connection) *hub {
	newH := &hub{
		HubName:     hubName,
		HubUsers:    make(map[string]string),
		broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}

	// register me in the hubmap
	if h != nil {
		h.hubMap[hubName] = newH
	}

	// Todo: Fix this when rethink can do uniques
	query := r.Table("hub").Filter(r.Row.Field("name").Eq(hubName))
	row, err := query.RunRow(dbSession)

	if err == nil && row.IsNil() {
		fmt.Println("row nil")
		_, err = r.Table("hub").Insert(newH).RunWrite(dbSession)
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

func (hb *hub) run() {
	for {
		select {
		case c := <-hb.register:
			hb.connections[c] = true
			hb.HubUsers[c.userID] = c.userName
			print, _ := json.MarshalIndent(hb, "", "  ")
			fmt.Println(string(print))
			r.Table("hub").Update(hb).RunWrite(dbSession)
		case c := <-hb.unregister:
			delete(hb.connections, c)
			delete(hb.HubUsers, c.userID)
			close(c.send)
			print, _ := json.MarshalIndent(hb, "", "  ")
			fmt.Println(string(print))
			// Removing doesn't work. TODO
			q := r.Table("hub").Get(hb.HubID).Replace(r.Row.Without(c.userID))
			fmt.Println(q)
			_, err := q.RunWrite(dbSession)
			fmt.Println(err)
		case m := <-hb.broadcast:
			for c := range hb.connections {
				select {
				case c.send <- m:
				default:
					close(c.send)
					delete(hb.connections, c)
				}
			}
		}
	}
}

// userDisconnect removes the user from all the hubs
func (hm *hubManager) userDisconnect(userID string) {

}
