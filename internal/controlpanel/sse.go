package controlpanel

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

//Functions for handling SSE to the control panel

type eventInfo struct {
	mu sync.RWMutex
	globalEventID uint64
	channels map[uint64]chan(string)
}

var events eventInfo = eventInfo{
	globalEventID: 0,
	channels: make(map[uint64]chan(string), 0),
}

//Function to set up channels for communication. Takes the channel that is used for writing to event stream.
func initSSE(w http.ResponseWriter, r *http.Request) {
	//Set headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	clientGone := r.Context().Done() //Channel for client disconnecting
	rc := http.NewResponseController(w) //Response controller

	id, writeC := registerSSEConnection()
	defer unregisterSSEConnection(id)

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

//Register log channel. Returns unique ID for this SSE connection
func registerSSEConnection() (uint64, chan(string)) {
	events.mu.Lock()
	//Get channel and new ID
	newEventChannel := make(chan(string), 16)
	newID := events.globalEventID
	events.globalEventID++

	//Add to map
	events.channels[newID] = newEventChannel
	events.mu.Unlock()

	go updateControlPanel() //Update the UI for new connection
	return newID, newEventChannel
}

func unregisterSSEConnection(id uint64) {
	events.mu.Lock()
	//Remove this channel from the map
	delete(events.channels, id)
	events.mu.Unlock()
}

//Sends message to control panel via SSE endpoint
func logMessage(message string) {
	events.mu.RLock()
	channels := make([]chan(string), 0)
	for _, v := range events.channels {
		if v != nil {
			channels = append(channels, v)
		}
	}
	events.mu.RUnlock()

	//Prefix every line with "data: "
	lines := strings.Split(message, "\n")
	dataLines := make([]string, len(lines))
	for i, l := range lines {
		dataLines[i] = "data: " + l
	}

	payload := fmt.Sprintf("event: log\n%s", strings.Join(dataLines, "\n"))

	for _, logC := range channels {
		select {
		case logC <- payload:
		default:
		}
	}
} 

func updateControlPanel() {
	events.mu.RLock()
	channels := make([]chan(string), 0)
	for _, v := range events.channels {
		if v != nil {
			channels = append(channels, v)
		}
	}
	events.mu.RUnlock()

	for _, logC := range channels {
		select {
		case logC <- "event: update\ndata:!":
		default:
		}
	}
}