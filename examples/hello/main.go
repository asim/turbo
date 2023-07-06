package main

import (
	"net/http"

	"github.com/asim/turbo"
)

func Index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<html><body><h1>Hello!</h1></body></html>`))
}

func Authed(w http.ResponseWriter, r *http.Request) {
	// this is an authenticated endpoint
	return
}

func main() {
	// create a new app
	app := turbo.New()

	// register an endpoint
	app.Register("/", turbo.Endpoint{
		Handler: Index,
		Private: false,
	})

	// register a authenticated endpoint
	app.Register("/authed", turbo.Endpoint{
		Handler: Authed,
		Private: true,
	})

	// run the app
	app.Run()
}
