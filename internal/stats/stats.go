package stats

import (
	"fmt"
	"net/http"

	"github.com/multimario_client/internal/store"
)

//This is the package for managing hosting the stats stream layout

//Init stats stream with information from the backend
type initStats struct {
	RaceCat string `json:"race_category"`
	RaceID int `json:"race_id"`
	Records []store.PlayerInfo `json:"records"`
	TimerValue string `json:"timer_value"`//hh:mm:ss format
	TimerRunning bool `json:"timer_running"`
}

var ip = "0.0.0.0"
var port = ":8080"

func InitStatsPage(layoutName string) {
	layoutPath := fmt.Sprintf("./layouts/%s", layoutName)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(layoutPath)))
	mux.Handle("/api.js", http.FileServer(http.Dir("./layouts")))

	//SSE
	mux.HandleFunc("GET /api/events", initSSE)
	
	http.ListenAndServe(ip+port, mux)
}