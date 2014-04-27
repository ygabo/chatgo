// This is from the martini-contrib example
// but this is using rethinkdb instead of sqlite3
// For learning purposes only.
package main

import (
	rethink "github.com/dancannon/gorethink"

	"code.google.com/p/go.crypto/bcrypt"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessionauth"
	"github.com/martini-contrib/sessions"

	"fmt"
	"net/http"
	"time"
)

var (
	INDEX_PAGE    = "/"
	LOGIN_PAGE    = "login"
	REGISTER_PAGE = "register"
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

func getLoginHandler(user sessionauth.User, r render.Render) {
	if user.IsAuthenticated() {
		r.Redirect(INDEX_PAGE)
		return
	}
	r.HTML(200, LOGIN_PAGE, nil)
}

func logoutHandler(session sessions.Session, user sessionauth.User, r render.Render) {
	sessionauth.Logout(session, user)
	r.Redirect(INDEX_PAGE)
}

func getRegisterHandler(user sessionauth.User, r render.Render) {
	if user.IsAuthenticated() {
		r.Redirect(INDEX_PAGE)
		return
	}
	r.HTML(200, REGISTER_PAGE, nil)
}

func postRegisterHandler(session sessions.Session, newUser User, r render.Render, req *http.Request) {

	if session.Get(sessionauth.SessionKey) != nil {
		fmt.Println("Logged in already! Logout first.")
		r.Redirect(INDEX_PAGE)
		return
	}

	var userInDb User
	query := rethink.Table("user").Filter(rethink.Row.Field("email").Eq(newUser.Email))
	row, err := query.RunRow(dbSession)

	if err == nil && !row.IsNil() {
		// Register, error case.
		if err := row.Scan(&userInDb); err != nil {
			fmt.Println("Error reading DB")
		} else {
			fmt.Println("User already exists. Redirecting to login.")
		}

		r.Redirect(sessionauth.RedirectUrl)
		return
	} else { // User doesn't exist, continue with registration.
		if row.IsNil() {
			fmt.Println("User doesn't exist. Registering...")
		} else {
			fmt.Println(err)
		}
	}

	// Try to compare passwords
	pass1Hash, _ := bcrypt.GenerateFromPassword([]byte(newUser.Password), bcrypt.DefaultCost)
	pass2String := req.FormValue("confirmpassword")
	passErr := bcrypt.CompareHashAndPassword(pass1Hash, []byte(pass2String))

	if passErr != nil {
		fmt.Println("Error, passwords don't match.", passErr)
	} else { // passwords are the same, insert user to db
		newUser.Password = string(pass1Hash)
		rethink.Table("user").Insert(newUser).RunWrite(dbSession)
		fmt.Println("Register done. Try to login.")
	}

	r.Redirect(sessionauth.RedirectUrl)
}

func postLoginHandler(session sessions.Session, userLoggingIn User, r render.Render, req *http.Request) {
	var userInDb User
	query := rethink.Table("user").Filter(rethink.Row.Field("email").Eq(userLoggingIn.Email))
	row, err := query.RunRow(dbSession)

	// TODO do flash errors
	if err == nil && !row.IsNil() {
		if err := row.Scan(&userInDb); err != nil {
			r.Redirect(sessionauth.RedirectUrl)
			return
		}
	} else {
		fmt.Println(err)
		r.Redirect(sessionauth.RedirectUrl)
		return
	}

	passErr := bcrypt.CompareHashAndPassword([]byte(userInDb.Password), []byte(userLoggingIn.Password))
	if passErr != nil {
		fmt.Println("Wrong Password")
		r.Redirect(sessionauth.RedirectUrl)
	} else {
		err := sessionauth.AuthenticateSession(session, &userInDb)
		if err != nil {
			fmt.Println("Wrong Auth")
			r.JSON(500, err)
		}
		params := req.URL.Query()
		redirect := params.Get(sessionauth.RedirectParam)
		r.Redirect(redirect)
	}
}
