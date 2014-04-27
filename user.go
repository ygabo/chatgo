// This is from the martini-contrib example
// but this is using rethinkdb instead of sqlite3
// For learning purposes only.
package main

import (
	// "errors"
	rethink "github.com/dancannon/gorethink"
	"github.com/martini-contrib/sessionauth"
	"time"
)

type User struct {
	Id            string          `form:"-" gorethink:"id,omitempty"`
	Email         string          `form:"email" gorethink:"email"`
	Password      string          `form:"password" gorethink:"password"`
	Username      string          `form:"-" gorethink:"username,omitempty"`
	Hubs          map[string]bool `form:"-" gorethink:"-"`
	Created       time.Time       `form:"-" gorethink:"-"`
	authenticated bool            `form:"-" gorethink:"-"`
}

// GetAnonymousUser should generate an anonymous user model
// for all sessions. This should be an unauthenticated 0 value struct.
func GenerateAnonymousUser() sessionauth.User {
	return &User{}
}

// Login will preform any actions that are required to make a user model
// officially authenticated.
func (u *User) Login() {
	// Update last login time
	// Add to logged-in user's list
	// etc ...
	u.authenticated = true
}

// Logout will preform any actions that are required to completely
// logout a user.
func (u *User) Logout() {
	// Remove from logged-in user's list
	// etc ...
	u.authenticated = false
}

func (u *User) IsAuthenticated() bool {
	return u.authenticated
}

func (u *User) UniqueId() interface{} {
	return u.Id
}

// Get user from the DB by id and populate it into 'u'
func (u *User) GetById(id interface{}) error {

	row, err := rethink.Table("user").Get(id).RunRow(dbSession)
	if err != nil {
		return err
	}
	if !row.IsNil() {
		if err := row.Scan(&u); err != nil {
			return err
		}
	}
	return nil
}
