package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/multimario_client/internal/store"
)

//Functions for handling SSE to the stats page

//Function to set up channels for communication. Takes the channel that is used for writing to event stream.
func initSSE(w http.ResponseWriter, r *http.Request) {
	//Set headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.Header().Set("Access-Control-Allow-Origin", "*")

	//Subscribe for internal updates
	stateCh, updateCh, cancel := store.Race.Subscribe(context.Background())
	defer cancel()
	
	rc := http.NewResponseController(w) //Response controller

	for {
		select {
		//When we get an update from state, get initialization json
		case state := <-stateCh:
			initJSON := getInitJSONString(&state)

			_, err := fmt.Fprintf(w, "data: %s\n\n", initJSON)
			if err != nil {
				return
			}

			err = rc.Flush() //Flush stream
			if err != nil {
				return
			}

		//On update, get update string and send it
		case update := <-updateCh:
			updateJSON := getUpdateJSONString(&update)

			_, err := fmt.Fprintf(w, "data: %s\n\n", updateJSON)
			if err != nil {
				return
			}

			err = rc.Flush() //Flush stream
			if err != nil {
				return
			}
		}
	}
}

func getUpdateJSONString(update *store.Update) string {
	switch (update.Type) {
	case store.PlayerCount:
		return getUpdatePlayerCountJSONString(update)
	case store.PlayerName:
		return getUpdatePlayerNameJSONString(update)
	case store.FinalTime:
		return getUpdatePlayerFinalTimeJSONString(update)
	case store.PlayerStatus:
		return getSetPlayerStatusJSONString(update)
	case store.StartTime:
		return getUpdateTimerValueJSONString(update)
	case store.TimerState:
		return getUpdateTimerStateJSONString(update)
	default:
		return ""
	}
}

//Gets race information and sends it to the stats stream as a JSON string via SSE
func getInitJSONString(state *store.State) string {
	//Convert into initStats instance and jsonify

	//Get records slice from player map
	recordsSlice := make([]store.PlayerInfo, 0)
	for _, v := range state.Players {
		if v != nil {
			recordsSlice = append(recordsSlice, *v)
		}
	}

	statsInit := initStats{
		RaceCat: state.Category,
		RaceID: state.RaceID,
		Records: recordsSlice,
		TimerValue: getTimerValueFromStartTime(state.Timer.StartTime),
		TimerRunning: state.Timer.TimerRunning,
	}

	initInfo := make(map[string]any)
	initInfo["init"] = statsInit

	//Convert to json byte slice
	jsonBytes, err := json.Marshal(initInfo)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

//Sends player count update to stats stream using twitch name
func getUpdatePlayerCountJSONString(update *store.Update) string {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_count"
	player["twitch_name"] = update.Player.PlayerTwitch
	player["num_collected"] = update.Player.NumCollected

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

//Sends player name update to stats stream using twitch name
func getUpdatePlayerNameJSONString(update *store.Update) string {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_name"
	player["twitch_name"] = update.Player.PlayerTwitch
	player["player_name"] = update.Player.Player

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

func getUpdatePlayerFinalTimeJSONString(update *store.Update) string {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_time"
	player["twitch_name"] = update.Player.PlayerTwitch
	player["time"] = update.Player.FinalTime

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
} 

func getSetPlayerStatusJSONString(update *store.Update) string {
	//Build map for this
	player := make(map[string]any)
	player["kind"] = "player_status"
	player["twitch_name"] = update.Player.PlayerTwitch
	player["status"] = update.Player.Status

	out := map[string]any {
		"update": player,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

//Sends timer update to stats stream using twitch name
func getUpdateTimerValueJSONString(update *store.Update) string {
	//Build map for this
	timer := make(map[string]any)
	timer["kind"] = "timer"
	timer["timer_value"] = getTimerValueFromStartTime(update.Timer.StartTime)

	out := map[string]any {
		"update": timer,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

func getUpdateTimerStateJSONString(update *store.Update) string {
	//Build map for this
	timer := make(map[string]any)
	timer["kind"] = "timer"
	timer["timer_running"] = update.Timer.TimerRunning

	out := map[string]any {
		"update": timer,
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

//Helper to convert start time into timer value
func getTimerValueFromStartTime(startTime int64) string {
	millis := time.Now().UnixMilli() - startTime

	duration := time.Duration(millis) * time.Millisecond

	hours := duration / time.Hour
	duration -= hours * time.Hour

	minutes := duration / time.Minute
	duration -= minutes * time.Minute

	seconds := duration / time.Second

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}