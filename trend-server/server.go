/*
	Gumball API in Go (Version 4)
	Uses MySQL
*/

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/codegangsta/negroni"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/unrolled/render"
)

func NewServer() *negroni.Negroni {
	formatter := render.New(render.Options{
		IndentJSON: true,
	})
	n := negroni.Classic()
	mx := mux.NewRouter()
	initRoutes(mx, formatter)
	n.UseHandler(mx)
	return n
}

// API Routes
func initRoutes(mx *mux.Router, formatter *render.Render) {
	mx.HandleFunc("/{id}", find(formatter)).Methods("GET")
}

func find(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		shortlink := mux.Vars(req)["id"]
		fmt.Println(shortlink)

		resp, _ := http.Get("http://localhost:9002/api/" + shortlink)

		var url string

		fmt.Println("Getting url from trend server..")
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		url = result["message"].(string)

		formatter.JSON(w, resp.StatusCode, url)
	}
}
