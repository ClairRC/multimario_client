package controlpanel

import (
	"fmt"
	"net/http"
)

//Functions for handling SSE to the control panel

//Function to set up channels for communication. Takes the channel that is used for writing to event stream.
func initSSE(w http.ResponseWriter, r *http.Request) {
	//Set headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	clientGone := r.Context().Done() //Channel for client disconnecting
	rc := http.NewResponseController(w) //Response controller

	writeC := make(chan(string), 5)
	registerSSEConnection(writeC)

	for {
		select {
		case msg := <-writeC:
			_, err := fmt.Fprintf(w, "%s\n\n", msg)
			if err != nil {
				return
			}

			err = rc.Flush() //Flush stream
			if err != nil {
				return
			}

		case <-clientGone:
			fmt.Println("client disconnected")
			return
		}
	}
}