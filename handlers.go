package main

import (
	"code.google.com/p/go.crypto/bcrypt"
	"errors"
	"fmt"
	rethink "github.com/dancannon/gorethink"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessionauth"
	"github.com/martini-contrib/sessions"
	"net/http"
	"time"
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

// func postTodoHandler(user sessionauth.User, todo Todo, r render.Render, req *http.Request) {
// 	todo.UserId = user.(*User).UniqueId().(string)
// 	var err error
// 	if todo.Id == "" {
// 		todo.Created = time.Now()
// 		_, err = rethink.Table("todo").Insert(todo).RunWrite(dbSession)
// 	} else {
// 		_, err = rethink.Table("todo").Update(todo).RunWrite(dbSession)
// 	}
// 	fmt.Println("ID:", todo.Id)
// 	if err != nil {
// 		fmt.Println("Error saving new todo", err)
// 		r.JSON(500, 0) // return empty
// 	} else {
// 		r.JSON(200, 1) // return OK
// 	}
// }

// func deleteTodoHandler(user sessionauth.User, r render.Render, parms martini.Params, req *http.Request) {
// 	todoID := parms["id"]
// 	var err error

// 	if todoID != "" {
// 		_, err = rethink.Table("todo").Get(todoID).Delete().RunWrite(dbSession)
// 	} else {
// 		err = errors.New("Invalid ID.")
// 	}
// 	if err != nil {
// 		r.JSON(500, 0) // return empty
// 	} else {
// 		r.JSON(200, 1)
// 	}
// }

// func getTodoPage(user sessionauth.User, r render.Render, req *http.Request) {
// 	items, err := user.(*User).GetMyTodoList()
// 	if err != nil {
// 		fmt.Println("Error getting todo list", err)
// 		items = nil
// 	} else {
// 		fmt.Println("Success, returning list.")
// 	}
// 	r.HTML(200, "todo", items)
// }

// func getTodoJSON(session sessions.Session, user sessionauth.User, r render.Render, parms martini.Params, req *http.Request) {
// 	var items *[]Todo
// 	var item *Todo
// 	var err error

// 	id := parms["id"]
// 	if id != "" {
// 		item, err = user.(*User).GetMyTodoByID(id)
// 	} else {
// 		items, err = user.(*User).GetMyTodoList()
// 	}

// 	if err != nil {
// 		fmt.Println("Error getting todo list", err)
// 		items = nil
// 	} else {
// 		fmt.Println("Success, returning list.")
// 	}

// 	if id != "" {
// 		r.JSON(200, item)
// 	} else {
// 		r.JSON(200, items)
// 	}
// }
