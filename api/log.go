package api

import (
	"io"
	"net/http"
	"os"
)

func LogsHandler(w http.ResponseWriter, r *http.Request) {
	// Set the content type of the response to "text/plain"
	w.Header().Set("Content-Type", "text/plain")

	// Read the logs from the log file and write them to the response
	f, err := os.Open("proxy.log")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
