package stats

import (
	"fmt"
	"net/http"
)

//Functions for handling SSE to the stats page

//Function to set up channels for communication. Takes the channel that is used for writing to event stream.
func initSSE(writeC chan(string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//Set headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		clientGone := r.Context().Done() //Channel for client disconnecting
		rc := http.NewResponseController(w) //Response controller

		for {
			select {
			case msg := <-writeC:
				_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
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
}