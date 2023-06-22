package main

import (
	"net/http"

	"github.com/asim/turbo"
)

func Index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<html><body><h1>Hello!</h1></body></html>`))
}

func main() {
	// create a new app
	app := turbo.New()

	// register an endpoint
	app.Register("/", Index)

	// run the app
	app.Run()
}
