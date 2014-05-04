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
	HubID   string `form:"-" gorethink:"id,omitempty""`
	HubName string `form:"name" gorethink:"name"`

	connections map[*connection]bool `form:"-" gorethink:"-"`
	broadcast   chan []byte          `form:"-" gorethink:"-"`
	register    chan *connection     `form:"-" gorethink:"-"`
	unregister  chan *connection     `form:"-" gorethink:"-"`
}

// Edges holds all the edges between the users and hubs, bidirectional
type Edges struct {
	User_to_hub map[string]map[string]*hub  // represents edge from users to hubs
	Hub_to_user map[string]map[string]*User // other way around
}

// hubuser represents the relationship between users and hubs
// type hubUser struct {
// 	HubID    string `gorethink:"hub_id"`
// 	UserID   string `gorethink:"user_id"`
// 	UserName string `gorethink:"user_name"`
// }

// hubManger is the in-memory hub manager
type hubManager struct {
	hubMap     map[string]*hub // maps hub IDs to the actual hub objects
	EdgeList   *Edges          // represents edges between users and hubs using their IDs
	defaultHub *hub            // default hub everyone connects to first
	// userHubMap map[string]map[string]bool // each user has a collection of hubs
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

	// create hub_user index
	_, err = r.Table("hub_user").IndexCreateFunc("hub_user_id", func(row r.RqlTerm) interface{} {
		return []interface{}{row.Field("hub_id"), row.Field("user_id")}
	}).Run(dbSession)
	fmt.Println("create index,", err)
	_, err = r.Table("hub_user").IndexWait().Run(dbSession)

	fmt.Println("wait index,", err)
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

			// // remember {hub, user} relationship
			// hu := &hubUser{HubID: hb.HubID, UserID: c.userID, UserName: c.userName}
			// _, err := r.Table("hub_user").Insert(hu).RunWrite(dbSession)

			// print, _ := json.MarshalIndent(hb, "", "  ")
			// fmt.Println(string(print), err)
		case c := <-hb.unregister:
			delete(hb.connections, c)
			delete(hb.HubUsers, c.userID)
			close(c.send)

			// // delete {hub, user} relationship
			// q := r.Table("hub_user").GetAllByIndex("hub_user_id", []interface{}{hb.HubID, c.userID}).Delete()
			// _, err := q.RunWrite(dbSession)
			// if err != nil {
			// 	fmt.Println(err)
			// }
		case m := <-hb.broadcast:
			for c := range hb.connections {
				select {
				case c.send <- m:
				default:
					close(c.send)
					delete(hb.connections, c)
					delete(hb.HubUsers, c.userID)

					// // delete {hub, user} relationship
					// q := r.Table("hub_user").GetAllByIndex("hub_user_id", []interface{}{hb.HubID, c.userID})
					// q = q.Delete()
					// _, err := q.RunWrite(dbSession)
					// if err != nil {
					// 	fmt.Println(err)
					// }
				}
			}
		}
	}
}

// Get user from the DB by id and populate it into 'u'
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

// userDisconnect removes the user from all the hubs
func (hm *hubManager) userDisconnect(userID string) {

}

// getHubByID return the hub with the corresponding ID
// nil if it doesn't exist
func (hm *hubManager) getHubByID(hubID string) *hub {
	for key, val := range hm.hubMap {
		if key == hubID {
			return val
		}
	}
	return nil
}

func (hm *hubManager) getAllHubsOfUser(userID string) []*hub {
	hubs := make([]*hub)

	if hm.EdgeList != nil && hm.EdgeList.User_to_hubs != nil {
		for currHub := range hm.EdgeList.User_to_hubs[userID] {
			hubs = append(hubs, currHub)
		}
	} else {
		return nil
	}

	return hubs
}

func (hm *hubManager) getAllUsersOfHub(hubID string) []*User {
	users := make([]*User)

	if hm.EdgeList != nil && hm.EdgeList.Hub_to_user != nil {
		for u := range hm.EdgeList.Hub_to_user[hubID] {
			users = append(users, u)
		}
	} else {
		return nil
	}

	return users
}

func (hm *hubManager) insertEdge(u *User, h *hub) {
	userID := u.Id
	hubID := h.HubID

	if hm.EdgeList == nil {
		hm.EdgeList = Edges{}
	}

	// Initialize needed structs
	if hm.EdgeList.Hub_to_user == nil {
		hm.EdgeList.Hub_to_user = make(map[string]map[string]*User)
	}
	if hm.EdgeList.User_to_hub == nil {
		hm.EdgeList.User_to_hub = make(map[string]map[string]*hub)
	}
	if hm.EdgeList.Hub_to_user[hubID] == nil {
		hm.EdgeList.Hub_to_user[hubId] = make(map[string]*User)
	}
	if hm.EdgeList.User_to_hub[userID] == nil {
		hm.EdgeList.User_to_hub[userID] = make(map[string]*hub)
	}

	hm.EdgeList.Hub_to_user[hubId][userID] = u
	hm.EdgeList.User_to_hub[userID][hubID] = h
}

// removeEdge deletes a relationship between a user and a hub
// nil hub means remove user from all his hubs
func (hm *hubManager) removeEdge(u *User, h *hub) error {
	if u == nil {
		return errors.New("User is nil.")
	}
	userID := u.Id
	var hubID string = nil

	if h != nil {
		hubID = h.HubID
		delete(hm.EdgeList.Hub_to_user[hubId], userID)
		delete(hm.EdgeList.User_to_hub[userID], hubID)
	} else { // else, delete user from all his hubs
		for hID := range hm.EdgeList.User_to_hub[userID] {
			delete(hm.EdgeList.Hub_to_user[hID], userID)
			delete(hm.EdgeList.User_to_hub[userID], hID)
		}
	}
}

func (hm *hubManager) removeEdgeByIDs(userID string, hubID string) error {
	u := User{}
	hb := hub{}
	u.GetById(userID)
	hb.GetById(hubID)
	hm.removeEdge(u, hb)
}
