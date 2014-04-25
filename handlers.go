package main

import (
	"code.google.com/p/go.crypto/bcrypt"
	"fmt"
	rethink "github.com/dancannon/gorethink"
	"github.com/gorilla/websocket"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessionauth"
	"github.com/martini-contrib/sessions"
	"log"
	"net/http"
)

func indexHandler(r render.Render) {
	r.HTML(200, "index", nil)
}

func getRoom(user sessionauth.User, r render.Render) {
	r.HTML(200, "index", nil)
}

func getLoginHandler(user sessionauth.User, r render.Render) {
	if user.IsAuthenticated() {
		r.Redirect("/")
		return
	}
	r.HTML(200, "login", nil)
}

func logoutHandler(session sessions.Session, user sessionauth.User, r render.Render) {
	sessionauth.Logout(session, user)
	r.Redirect("/")
}

func getRegisterHandler(user sessionauth.User, r render.Render) {
	if user.IsAuthenticated() {
		r.Redirect("/")
		return
	}
	r.HTML(200, "register", nil)
}

func postRegisterHandler(session sessions.Session, newUser User, r render.Render, req *http.Request) {

	if session.Get(sessionauth.SessionKey) != nil {
		fmt.Println("Logged in already! Logout first.")
		r.HTML(200, "index", nil)
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

func getHub(r render.Render) {
	r.HTML(200, "room", nil)
}

func sendAll(msg []byte) {
	for conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			delete(connections, conn)
			conn.Close()
		}
	}
}

func wsHandler(w http.ResponseWriter, user sessionauth.User, r *http.Request) {
	// Taken from gorilla's website
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		return
	} else if err != nil {
		return
	}

	fmt.Println("hello")
	connections[conn] = true

	for {
		// Blocks until a message is read
		_, msg, err := conn.ReadMessage()
		if err != nil {
			delete(connections, conn)
			conn.Close()
			return
		}
		log.Println(string(msg))
		sendAll(msg)
	}
}
