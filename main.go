// Building on top of the auth example is the todo app example.

// This from the martini-contrib sessionauth example,
// but this is using RethinkDB instead of sqlite3. For personal learning purposes only.
package main

import (
	"code.google.com/p/go.crypto/bcrypt"
	"fmt"
	rethink "github.com/dancannon/gorethink"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessionauth"
	"github.com/martini-contrib/sessions"
	"log"
)

var (
	dbSession *rethink.Session
)

func init() {
	// var dbError error
	// dbSession, dbError = rethink.Connect(rethink.ConnectOpts{
	// 	Address:  "localhost:28015",
	// 	Database: "todo"})
	// if dbError != nil {
	// 	log.Fatalln(dbError.Error())
	// }

	// // Testing purposes: query myself.
	// me := User{Email: "yelnil@example.com"}
	// hpass, _ := bcrypt.GenerateFromPassword([]byte("qwe"), bcrypt.DefaultCost)
	// me.Password = string(hpass)
	// row, err := rethink.Table("user").Filter(rethink.Row.Field("email").Eq(me.Email)).RunRow(dbSession)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// // I don't exist, insert me.
	// if row.IsNil() {
	// 	rethink.Table("user").Insert(me).RunWrite(dbSession)
	// 	return
	// }
	//row.Scan(&me)
	//todo := Todo{UserId: me.UniqueId().(string), Body: "Finish todo app.", Completed: false}
	//rethink.Table("todo").Insert(todo).RunWrite(dbSession)
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
	m.Get("/login", getLoginHandler)
	m.Get("/register", getRegisterHandler)
	m.Get("/logout", sessionauth.LoginRequired, logoutHandler)
	m.Post("/login", binding.Bind(User{}), postLoginHandler)
	m.Post("/register", binding.Bind(User{}), postRegisterHandler)

	m.Get("/room", sessionauth.LoginRequired, wsHandler)

	// m.Get("/todo.json", sessionauth.LoginRequired, getTodoJSON)
	// m.Get("/todo.json/:id", sessionauth.LoginRequired, getTodoJSON)
	// m.Post("/todo.json", sessionauth.LoginRequired, binding.Bind(Todo{}), postTodoHandler)
	// m.Post("/todo.json/:id", sessionauth.LoginRequired, binding.Bind(Todo{}), postTodoHandler)
	// m.Delete("/todo.json/:id", sessionauth.LoginRequired, deleteTodoHandler)

	m.Use(martini.Static("static"))
	m.Run()
}
