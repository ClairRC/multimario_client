package stats

import (
	"encoding/json"
	"fmt"
	"net/http"
)

//This is the package for managing hosting the stats stream layout

//Struct for player info that gets sent to stats stream
type PlayerInfo struct {
	//Similar to mmapi.records, but adds some Twitch info for stats stream to use
	NumCollected float64 `json:"num_collected"`
	Player string `json:"player_name"`
	PlayerTwitch string `json:"twitch_name"`
	FinalTime string `json:"time"`
	Estimate string `json:"estimate"`
	Status string `json:"status"`
	ProfilePictureURL string `json:"pfp_url"`
}

//Init stats stream with information from the backend
type initStats struct {
	RaceCat string `json:"race_category"`
	RaceID int `json:"race_id"`
	Records []PlayerInfo `json:"records"`
	TimerValue string `json:"timer_value"`//hh:mm:ss format
	TimerRunning bool `json:"timer_running"`
}

var ip = "0.0.0.0"
var port = ":8080"

var currentRaceID = -1 //Package level variable for race ID. Maybe should be put in some context struct? Idk.
var messageQueue = make(chan(string), 100) //Queue for messages
var logC = make(chan(string)) //Channel for sending messages over SSE

func InitStatsPage(layoutName string) {
	layoutPath := fmt.Sprintf("./layouts/%s", layoutName)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(layoutPath)))
	mux.Handle("/api.js", http.FileServer(http.Dir("./layouts")))

	//SSE
	mux.HandleFunc("GET /api/events", initSSE(logC))

	//Begin processing message queue
	go processMessageQueue(messageQueue)

	http.ListenAndServe(ip+port, mux)
}

//Gets race information and sends it to the stats stream as a JSON string via SSE
func StartTrackingRace(raceID int, players []PlayerInfo, raceCategory string, startingTimerValue string, startTimer bool) error {
	//Convert into initStats instance and jsonify
	statsInit := initStats{
		RaceCat: raceCategory,
		RaceID: raceID,
		Records: players,
		TimerValue: startingTimerValue,
		TimerRunning: startTimer,
	}

	initInfo := make(map[string]any)
	initInfo["init"] = statsInit

	//Convert to json byte slice
	jsonBytes, err := json.Marshal(initInfo)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
}

//Sends player count update to stats stream using twitch name
func UpdatePlayerCount(playerTwitchName string, numCollected int) error {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_count"
	player["twitch_name"] = playerTwitchName
	player["num_collected"] = numCollected

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
}

//Sends player name update to stats stream using twitch name
func UpdatePlayerName(playerTwitchName string, newName string) error {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_name"
	player["twitch_name"] = playerTwitchName
	player["player_name"] = newName

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
}

func UpdatePlayerFinalTime(playerTwitchName string, newTime string) error {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_time"
	player["twitch_name"] = playerTwitchName
	player["time"] = newTime

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
} 

func SetPlayerStatus(playerTwitchName string, statusText string) error {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_status"
	player["twitch_name"] = playerTwitchName
	player["status"] = statusText

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
}

//Sends timer update to stats stream using twitch name
func UpdateTimerValue(timerValue string) error {
	//Build map for this
	timer := make(map[string]any)
	timer["kind"] = "timer"
	timer["timer_value"] = timerValue

	out := map[string]any {
		"update": timer,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
}

func UpdateTimerState(timerStart bool) error {
	//Build map for this
	timer := make(map[string]any)
	timer["kind"] = "timer"
	timer["timer_running"] = timerStart

	out := map[string]any {
		"update": timer,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}

	sendInfoToStats(string(jsonBytes))

	return nil
}

//Helper function to send information. This guarantees only one signal can be in flight at a time
func sendInfoToStats(info string) {
	//TODO: handle concurrent init requests
	messageQueue <- info
}

func processMessageQueue(messageQueue chan string) {
	for msg := range messageQueue {
		logC <- msg
	}
}

//Sets the current race ID and alert the stats page
func SetCurrentRaceID(raceID int) {
	currentRaceID = raceID
}