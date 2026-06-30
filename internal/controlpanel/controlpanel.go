package controlpanel

import "net/http"

//Package for handling the web-based control panel that the end user can use

//This is where the UI display stuff is hosted. For now this should just be local host, but if this bot is ever
//run remotely, this can change
var ip = "localhost"
var port = ":8081"
var logC = make(chan(string)) //Channel for logging events

func InitControlPanel() {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./ui/static")))
	mux.HandleFunc("GET /api/upcoming_races", sendUpcomingRaces)
	mux.HandleFunc("GET /api/in_progress_race", sendInProgressRace)
	mux.HandleFunc("POST /api/start_race", startRace)
	mux.HandleFunc("POST /api/finish_race", finishRace)
	mux.HandleFunc("POST /api/pause_race", pauseRace)

	//SSE
	mux.HandleFunc("GET /api/events", initSSE(logC))

	http.ListenAndServe(ip+port, mux)
}

//Sends message to control panel via SSE endpoint
func logMessage(message string) {
	logC <- message
} 