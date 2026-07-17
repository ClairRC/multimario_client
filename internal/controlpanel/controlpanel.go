package controlpanel

import (
	"net/http"
)

//Package for handling the web-based control panel that the end user can use

//This is where the UI display stuff is hosted. For now this should just be local host, but if this bot is ever
//run remotely, this can change
var ip = "0.0.0.0"
var port = ":8081"
var logC = make(chan(string)) //Channel for logging events

func InitControlPanel() {
	//Register commands
	initCommands()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./ui/static")))
	mux.HandleFunc("GET /api/upcoming_races", sendUpcomingRaces)
	mux.HandleFunc("GET /api/past_races", sendCompletedRaces)
	mux.HandleFunc("GET /api/in_progress_race", sendInProgressRace)
	mux.HandleFunc("GET /api/connected_to_twitch", isConnectedToTwitch)
	mux.HandleFunc("POST /api/select_race", selectRace)
	mux.HandleFunc("POST /api/start_race", startRace)
	mux.HandleFunc("POST /api/finish_race", finishRace)
	mux.HandleFunc("POST /api/reset_race", resetRace)
	mux.HandleFunc("POST /api/connect_to_twitch", connectToTwitchChat)
	mux.HandleFunc("POST /api/disconnect_from_twitch", disconnectFromTwitchChat)
	mux.HandleFunc("POST /api/submit_command", parseCommand)

	//SSE
	mux.HandleFunc("GET /api/events", initSSE(logC))

	http.ListenAndServe(ip+port, mux)
}

//Sends message to control panel via SSE endpoint
func logMessage(message string) {
	logC <- message
} 