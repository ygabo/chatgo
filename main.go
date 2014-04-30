// Building on top of the auth example is the todo app example.

// This from the martini-contrib sessionauth example,
// but this is using RethinkDB instead of sqlite3. For personal learning purposes only.
package main

import (
	"fmt"
	"log"
	"os"

	rethink "github.com/dancannon/gorethink"

	"code.google.com/p/go.crypto/bcrypt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessionauth"
	"github.com/martini-contrib/sessions"
)

var dbSession *rethink.Session

func init() {
	dbAddress := os.Getenv("RETHINKDB_ADDRESS")
	dbName := os.Getenv("RETHINK_TODO_DB")

	dbSession, dbError = rethink.Connect(rethink.ConnectOpts{
		Address:  dbAddress,
		Database: dbName})
	if dbError != nil {
		log.Fatalln(dbError.Error())
	}
}

func indexHandler(user sessionauth.User, r render.Render) {
	r.HTML(200, "index", user.(*User))
}

func main() {
	store := sessions.NewCookieStore([]byte("secret123"))
	m := martini.Classic()

	templateOptions := render.Options{}
	templateOptions.Delims.Left = "#{"
	templateOptions.Delims.Right = "}#"
	m.Use(render.Renderer(templateOptions))

	store.Options(sessions.Options{MaxAge: 0})
	m.Use(sessions.Sessions("my_session", store))

	// Every request is bound with empty user. If there's a session,
	// that empty user is filled with appopriate data
	m.Use(sessionauth.SessionUser(GenerateAnonymousUser))
	sessionauth.RedirectUrl = "/login"
	sessionauth.RedirectParam = "next"

	m.Get("/", indexHandler)
	m.Get("/login", getLoginPage)
	m.Get("/edit", getEditPage)
	m.Get("/register", getRegisterPage)
	m.Get("/logout", sessionauth.LoginRequired, logoutHandler)
	m.Post("/login", binding.Bind(User{}), postLoginHandler)
	m.Post("/register", binding.Bind(User{}), postRegisterHandler)
	m.Post("/edit", sessionauth.LoginRequired, binding.Bind(User{}), postEditHandler)

	m.Get("/room", sessionauth.LoginRequired, wsHandler)
	m.Get("/hub", sessionauth.LoginRequired, getHub)

	m.Use(martini.Static("static"))
	m.Run()
}
