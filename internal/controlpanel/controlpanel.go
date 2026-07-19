package controlpanel

import (
	"fmt"
	"net/http"
)

//Package for handling the web-based control panel that the end user can use

//This is where the UI display stuff is hosted. For now this should just be local host, but if this bot is ever
//run remotely, this can change
var ip = "0.0.0.0"
var port = ":8081"
var logChannels = make([]chan(string), 0)

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

	http.ListenAndServe(ip+port, mux)
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

//Register log channel
func registerSSEConnection(ch chan(string)) {
	logChannels = append(logChannels, ch)

	updateControlPanel() //Update the UI
}

//Sends message to control panel via SSE endpoint
func logMessage(message string) {
	for _, logC := range logChannels {
		logC <- fmt.Sprintf("event: log\ndata: %s", message)
	}
} 

func updateControlPanel() {
	for _, logC := range logChannels {
		logC <- "event: update\ndata:!"
	}
}