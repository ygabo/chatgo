// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"

	r "github.com/dancannon/gorethink"
)

// hub maintains the set of active connections and broadcasts messages to the
// connections.
type hub struct {
	HubID     string         `form:"-" gorethink:"id"`
	HubName   string         `form:"name" gorethink:"name"`
	HubAdmins map[string]int `gorethink:"admins"`

	connections map[*connection]bool `form:"-" gorethink:"-"`
	broadcast   chan []byte          `form:"-" gorethink:"-"`
	register    chan *connection     `form:"-" gorethink:"-"`
	unregister  chan *connection     `form:"-" gorethink:"-"`
}

// Edges holds all the edges between the users and hubs, bidirectional
type Edges struct {
	Hub_to_users map[*hub]map[*connection]bool // one-to-many hub  -> conns
	User_to_hubs map[*connection]map[*hub]bool // one-to-many conn -> hubs
}

// hubConnMsg is the message type passed to the hubmanager
type hubConnMsg struct {
	Hub *hub
	Con *connection

	HubID string
	Msg   []byte
}

// hubManger is the in-memory hub manager
type hubManager struct {
	HubMap     map[string]*hub // maps hub IDs to the actual hub objects
	EdgeMap    *Edges          // represents edges between users and hubs
	DefaultHub *hub            // default hub everyone connects to first

	newHub     chan hubConnMsg
	addEdge    chan hubConnMsg
	remEdge    chan hubConnMsg
	bCastToHub chan hubConnMsg
}

var h *hubManager

func init() {
	h = &hubManager{
		HubMap:  make(map[string]*hub),
		EdgeMap: &Edges{},

		newHub:     make(chan hubConnMsg, 256),
		addEdge:    make(chan hubConnMsg, 2048),
		remEdge:    make(chan hubConnMsg, 2048),
		bCastToHub: make(chan hubConnMsg, 2048),
	}

	var err error
	h.DefaultHub, err = newHub("default", nil)

	if err != nil {
		fmt.Println("Default insert error, still running hub.", err)
	}

	// create index
	_, err = r.Table("hub").IndexCreate("name").Run(dbSession)
	fmt.Println("create index name error: ", err)
	_, err = r.Table("user").IndexCreate("email").Run(dbSession)
	fmt.Println("create index user email error: ", err)

	go h.run()
}

// newHub return's a new hub object
// It takes in a connection that will be inserted into the hub if not nil
func newHub(hubName string, con *connection) (*hub, error) {
	newH := &hub{
		HubName:   hubName,
		HubAdmins: make(map[string]int),

		broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}

	err := newH.GetByName(hubName)

	if newH.HubID == "" && err == nil { // hub not in DB, insert
		_, err = r.Table("hub").Insert(newH).RunWrite(dbSession)
		fmt.Println("hub not in db, insert", newH)
	}

	if err != nil {
		fmt.Println("Error newHub", err)
		return nil, err
	}

	// register new hub in the hubmap
	msg := hubConnMsg{Con: con, Hub: newH}

	if h != nil {
		h.newHub <- msg
	}

	if con != nil {
		h.addEdge <- msg
	}

	go newH.run()

	return newH, nil
}

func (hb *hub) run() {
	for {
		select {
		case r := <-hb.register:
			hb.connections[r] = true
		case unr := <-hb.unregister:
			delete(hb.connections, unr)
		case m := <-hb.broadcast:
			for c := range hb.connections {
				select {
				case c.send <- m:
				default:
					h.remEdge <- hubConnMsg{Con: c, Hub: hb}
				}
			}
		}
	}
}

// Get hub from the DB by id and populate it into 'gb'
// This is not a complete representation of hub, since it
// will only have ID and name after querying. (no conns or anything)
// The real hub is in h.hubMap[] which is an in-memory store
func (hb *hub) GetById(id string) error {

	row, err := r.Table("hub").Get(id).RunRow(dbSession)
	if err != nil {
		return err
	}
	if !row.IsNil() {
		if err := row.Scan(&hb); err != nil {
			return err
		}
	}
	return nil
}

func (hb *hub) GetByName(hbName string) error {
	row, err := r.Table("hub").Filter(r.Row.Field("name").Eq(hbName)).RunRow(dbSession)

	if err != nil {
		fmt.Println("Error getbyname filter.")
		return err
	}
	if !row.IsNil() {
		if err := row.Scan(&hb); err != nil {
			fmt.Println("Error scanning hub from db.")
			return err
		}
	}
	return nil
}

func (hm *hubManager) run() {
	for {
		select {
		case n := <-hm.newHub:
			hm.HubMap[n.Hub.HubID] = n.Hub
		case a := <-hm.addEdge:
			hm.insertEdge(a.Con, a.Hub)
		case r := <-hm.remEdge:
			hm.removeEdge(r.Con, r.Hub)
		case b := <-hm.bCastToHub:
			hm.HubMap[b.HubID].broadcast <- b.Msg
		}
	}
}

func (hm *hubManager) insertEdge(c *connection, hb *hub) {

	if hm.EdgeMap == nil {
		hm.EdgeMap = &Edges{}
	}

	// Initialize needed structs
	if hm.EdgeMap.Hub_to_users == nil {
		hm.EdgeMap.Hub_to_users = make(map[*hub]map[*connection]bool)
	}
	if hm.EdgeMap.User_to_hubs == nil {
		hm.EdgeMap.User_to_hubs = make(map[*connection]map[*hub]bool)
	}
	if hm.EdgeMap.Hub_to_users[hb] == nil { // use the hb's connection map
		if hb.connections == nil {
			hb.connections = make(map[*connection]bool)
		}
		hm.EdgeMap.Hub_to_users[hb] = hb.connections
	}
	if hm.EdgeMap.User_to_hubs[c] == nil {
		hm.EdgeMap.User_to_hubs[c] = make(map[*hub]bool)
	}

	hb.register <- c
	hm.EdgeMap.User_to_hubs[c][hb] = true
}

// removeEdge deletes a relationship between a user and a hub
// nil hub means remove user from all his hubs
func (hm *hubManager) removeEdge(c *connection, hb *hub) error {
	if c == nil {
		return errors.New("conn is nil.")
	}

	if hb != nil {
		hb.unregister <- c
		delete(hm.EdgeMap.User_to_hubs[c], hb)
	} else { // else, delete user from all his hubs
		for hh := range hm.EdgeMap.User_to_hubs[c] {
			hh.unregister <- c
			delete(hm.EdgeMap.User_to_hubs[c], hh)
		}
	}
	return nil
}
