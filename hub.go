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
	Hub_to_users map[*hub]*map[*connection]bool // one-to-many hub  -> conns
	User_to_hubs map[*connection]*map[*hub]bool // one-to-many conn -> hubs
}

// hubManger is the in-memory hub manager
type hubManager struct {
	hubMap     map[string]*hub // maps hub IDs to the actual hub objects
	EdgeMap    *Edges          // represents edges between users and hubs
	defaultHub *hub            // default hub everyone connects to first
}

var h *hubManager

func init() {
	h = &hubManager{
		hubMap:     make(map[string]*hub),
		EdgeMap:    Edges{},
		defaultHub: newHub("default", nil),
	}
	// since h is still nil when making default hub
	h.hubMap[h.defaultHub.HubID] = h.defaultHub

	if err != nil {
		fmt.Println("Default insert error, still running hub.", err)
	}

	// create index
	_, err = r.Table("hub").IndexCreate("name").Run(dbSession)
	fmt.Println("create index name error: ", err)
	_, err = r.Table("user").IndexCreate("email").Run(dbSession)
	fmt.Println("create index user email error: ", err)

	go h.defaultHub.run()
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

	if con != nil {
		newH.connections[con] = true
	}

	// register new hub in the hubmap
	if h != nil {
		h.hubMap[newH.HubID] = newH
	}

	return newH, nil
}

func (hb *hub) run() {
	for {
		select {
		case c := <-hb.register:
			h.insertEdge(c, hb)
		case c := <-hb.unregister:
			h.removeEdge(c, hb)
		case m := <-hb.broadcast:
			for c := range hb.connections {
				select {
				case c.send <- m:
				default:
					h.removeEdge(c, hb)
				}
			}
		}
	}
}

// Get hub from the DB by id and populate it into 'gb'
// This is not a complete representation of hub, since it
// will only have ID and name after querying. (no conns or anything)
// The real hub is in h.hubMap[] which is an in-memory store
func (hb *hub) GetById(id interface{}) error {

	row, err := rethink.Table("hub").Get(id).RunRow(dbSession)
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

// userDisconnect removes the user from all the hubs
func (hm *hubManager) userDisconnect(userID *string) {
	hm.removeEdgeByIDs(userID, nil)
}

// getHubByID return the hub with the corresponding ID
// nil if it doesn't exist
func (hm *hubManager) getHubByID(hubID *string) *hub {
	for key, val := range hm.hubMap {
		if key == hubID {
			return val
		}
	}
	return nil
}

func (hm *hubManager) getAllHubsOfUser(userID *string) []*hub {
	hubs := make([]*hub)

	if hm.EdgeMap != nil && hm.EdgeMap.User_to_hubs != nil {
		for currHub := range hm.EdgeMap.User_to_hubs[connMap[userID]] {
			hubs = append(hubs, currHub)
		}
	} else {
		return nil
	}

	return hubs
}

func (hm *hubManager) getAllUsersOfHub(hb *hub) []*User {
	users := make([]*User)

	if hm.EdgeMap != nil && hm.EdgeMap.Hub_to_users != nil {
		for conn := range hm.EdgeMap.Hub_to_conns[hb] {
			users = append(users, &User{Id: conn.userID, Username: conn.userName})
		}
	} else {
		return nil
	}

	return users
}

func (hm *hubManager) insertEdge(c *connection, hb *hub) {
	hubID := h.HubID

	if hm.EdgeMap == nil {
		hm.EdgeMap = Edges{}
	}

	// Initialize needed structs
	if hm.EdgeMap.Hub_to_users == nil {
		hm.EdgeMap.Hub_to_users = make(map[*hub]map[*connection]bool)
	}
	if hm.EdgeMap.User_to_hubs == nil {
		hm.EdgeMap.User_to_hubs = make(map[*connection]map[*hub]bool)
	}
	if hm.EdgeMap.Hub_to_users[hb] == nil { // use the hb's connection map
		hm.EdgeMap.Hub_to_users[hb] = &hb.connections
	}
	if hm.EdgeMap.User_to_hubs[c] == nil {
		hm.EdgeMap.User_to_hubs[c] = make(map[*hub]bool)
	}

	hm.EdgeMap.Hub_to_users[hb][c] = true
	hm.EdgeMap.User_to_hubs[c][hb] = true
}

func (hm *hubManager) insertEdgeByID(uId *string, hId *string) {
	con := connMap[uId]

	if hm.hubMap[hId] == nil {
		hm.hubMap[hId] = newHub("", nil)
	}

	hm.insertEdge(con, hm.hubMap[hId])
}

// removeEdge deletes a relationship between a user and a hub
// nil hub means remove user from all his hubs
func (hm *hubManager) removeEdge(c *connection, h *hub) error {
	if c == nil {
		return errors.New("conn is nil.")
	}
	userID := c.userID
	var hubID string

	if h != nil {
		hubID = h.HubID
		delete(hm.EdgeMap.Hub_to_users[hub], c)
		delete(hm.EdgeMap.User_to_hubs[c], hub)
	} else { // else, delete user from all his hubs
		for hh := range hm.EdgeMap.User_to_hubs[c] {
			delete(hm.EdgeMap.Hub_to_users[hh], c)
			delete(hm.EdgeMap.User_to_hubs[c], hh)
		}
	}
	return nil
}

// removeEdgeByIDs is a wrapper on removeEdge if you want to pass in the IDs
func (hm *hubManager) removeEdgeByIDs(userID *string, hubID *string) error {
	c := connMap[userID]

	var hb *hub = nil
	if hubID != nil {
		hb = hm.hubMap[hubID]
	}

	hm.removeEdge(c, hb)
}
