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
	Id            string `form:"-" gorethink:"id,omitempty"`
	Email         string `form:"email" gorethink:"email"`
	Password      string `form:"password" gorethink:"password"`
	Username      string `form:"-" gorethink:"username,omitempty"`
	Created       time.Time
	authenticated bool `form:"-" gorethink:"-"`
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

// // Get the todo list associated with a user.
// func (u *User) GetMyTodoList() (*[]Todo, error) {
// 	if !u.IsAuthenticated() {
// 		// TODO, distinguish between my own todo and others.
// 		return nil, errors.New("Not authenticated.")
// 	}

// 	//.(string) means it's casting the id into a string. (it returns an interface)
// 	query := rethink.Table("todo").Filter(rethink.Row.Field("user_id").Eq(u.UniqueId().(string)))
// 	query = query.OrderBy(rethink.Asc("Created"))
// 	rows, err := query.Run(dbSession)

// 	if err != nil {
// 		return nil, err
// 	}

// 	list := []Todo{}

// 	for rows.Next() {
// 		var item Todo
// 		err := rows.Scan(&item)
// 		if err != nil {
// 			return &list, err
// 		}
// 		list = append(list, item)
// 	}
// 	return &list, nil
// }

// // Get a todo item by id associated by this user
// func (u *User) GetMyTodoByID(todoID string) (*Todo, error) {
// 	if !u.IsAuthenticated() {
// 		// TODO, distinguish between my own todo and others.
// 		// If I want others to see my todo
// 		return nil, errors.New("Not authenticated.")
// 	}

// 	query := rethink.Table("todo").Filter(rethink.Row.Field("id").Eq(todoID))
// 	row, err := query.RunRow(dbSession)

// 	if row.IsNil() || err != nil {
// 		return nil, err
// 	}

// 	todo := Todo{}
// 	if err := row.Scan(&todo); err != nil {
// 		return nil, err
// 	}

// 	return &todo, nil
// }
