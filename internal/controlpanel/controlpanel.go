package controlpanel

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/pkg/browser"
)

//Package for handling the web-based control panel that the end user can use

//This is where the UI display stuff is hosted. For now this should just be local host, but if this bot is ever
//run remotely, this can change
var ip = "0.0.0.0"
var port = ":8081"

var eventsMu sync.RWMutex
var globalID uint64 = 0
var eventChannels = make(map[uint64]chan(string), 0)

func InitControlPanel() {
	//Register commands
	initCommands()

	//Load saved schedule
	err := Schedule.loadSchedule(scheduleCachePath)
	if err != nil {
		logMessage(fmt.Sprintf("Unable to load scheduled race: %s", err.Error()))
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./ui/static")))
	mux.HandleFunc("GET /api/upcoming_races", sendUpcomingRacesHandler)
	mux.HandleFunc("GET /api/past_races", sendCompletedRacesHandler)
	mux.HandleFunc("GET /api/in_progress_race", sendInProgressRaceHandler)
	mux.HandleFunc("GET /api/connected_to_twitch", isConnectedToTwitchHandler)
	mux.HandleFunc("POST /api/select_race", sendUpdateMiddleware(selectRaceHandler))
	mux.HandleFunc("POST /api/start_race", sendUpdateMiddleware(startRaceHandler))
	mux.HandleFunc("POST /api/finish_race", sendUpdateMiddleware(finishRaceHandler))
	mux.HandleFunc("POST /api/reset_race", sendUpdateMiddleware(resetRaceHandler))
	mux.HandleFunc("POST /api/connect_to_twitch", sendUpdateMiddleware(connectToTwitchChatHandler))
	mux.HandleFunc("POST /api/disconnect_from_twitch", sendUpdateMiddleware(disconnectFromTwitchChatHandler))
	mux.HandleFunc("POST /api/submit_command", sendUpdateMiddleware(parseCommandHandler))

	//SSE
	mux.HandleFunc("GET /api/events", initSSE)

	fmt.Printf("Hosting control panel on http://localhost%s\n", port)
	go http.ListenAndServe(ip+port, mux)
	browser.OpenURL(fmt.Sprintf("http://localhost%s", port))
}

//Middleware to send latest updates to each channel whenever they call an endpoint
func sendUpdateMiddleware(handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	//TODO: This middleware/SSE pattern means that the UI doesn't need to actually send requests constantly for updates,
	//it can really just get served them, but it works
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
		updateControlPanel()
	}
}

//Register log channel. Returns unique ID for this SSE connection
func registerSSEConnection() (uint64, chan(string)) {
	eventsMu.Lock()
	//Get channel and new ID
	newEventChannel := make(chan(string), 16)
	newID := globalID
	globalID++

	//Add to map
	eventChannels[newID] = newEventChannel
	eventsMu.Unlock()

	go updateControlPanel() //Update the UI for new connection
	return newID, newEventChannel
}

func unregisterSSEConnection(id uint64) {
	eventsMu.Lock()
	//Remove this channel from the map
	delete(eventChannels, id)
	eventsMu.Unlock()
}

//Sends message to control panel via SSE endpoint
func logMessage(message string) {
	eventsMu.RLock()
	channels := make([]chan(string), 0)
	for _, v := range eventChannels {
		if v != nil {
			channels = append(channels, v)
		}
	}
	eventsMu.RUnlock()

	for _, logC := range channels {
		select {
		case logC <- fmt.Sprintf("event: log\ndata: %s", message):
		default:
		}
	}
} 

func updateControlPanel() {
	eventsMu.RLock()
	channels := make([]chan(string), 0)
	for _, v := range eventChannels {
		if v != nil {
			channels = append(channels, v)
		}
	}
	eventsMu.RUnlock()

	for _, logC := range channels {
		select {
		case logC <- "event: update\ndata:!":
		default:
		}
	}
}