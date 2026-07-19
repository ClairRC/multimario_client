package controlpanel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch/chat"
)

//Contains endpoints for the control panel to use to communicate with the bot

/*
* GET
* Returns
* {
*	races: [
*		{
*			category
*			date
*			id
*		}
*	]
* }
 */

func sendUpcomingRacesHandler(w http.ResponseWriter, r *http.Request) {
    //Get upcoming races
	w.Header().Set("Content-Type", "application/json")

	races, err := mmapi.GetUpcomingRaces()
	if err != nil {
		logMessage(fmt.Sprintf("ERROR: %s", err.Error()))
		return
	}

	out := make(map[string]any)
	out["races"] = races

	json.NewEncoder(w).Encode(&out)
}

/*
* GET
* Returns
* {
*	races: [
*		{
*			category
*			date
*			id
*		}
*	]
* }
 */

func sendCompletedRacesHandler(w http.ResponseWriter, r *http.Request) {
    //Get completed races
	w.Header().Set("Content-Type", "application/json")

	races, err := mmapi.GetCompletedRaces()
	if err != nil {
		logMessage(fmt.Sprintf("ERROR: %s", err.Error()))
		return
	}

	out := make(map[string]any)
	out["races"] = races

	json.NewEncoder(w).Encode(&out)
}

/*
* GET
* Returns
* {
*	race: {
*		category
*		date
*		id
*	}
* }
*/
func sendInProgressRaceHandler(w http.ResponseWriter, r *http.Request) {
	//Get in progress race
	w.Header().Set("Content-Type", "application/json")

	//Prioritize sending actual in progress race
	race, err := mmapi.GetInProgressRace()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	//No in progress race, get stored race
	if race == nil {
		race = store.Race.GetStoredRaceInfo()
	}

	//Get output
	out := make(map[string]any)
	out["race"] = nil

	if race != nil {
		out["race"] = &race
	}

	json.NewEncoder(w).Encode(&out)
}

/*
* GET
* Returns
* {
*	connected: bool
* }
*/
func isConnectedToTwitchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"connected": chat.Client.IsConnectedToTwitch()})
}

/*
* POST
*/
func connectToTwitchChatHandler(w http.ResponseWriter, r *http.Request) {
	//Get twitch channels from storage
	err := connectToTwitchChat()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}

/*
* POST
* Takes race_id in URL
*/

//Function that takes race information and then passes that along to the stats stream
func selectRaceHandler(w http.ResponseWriter, r *http.Request) {
	//Gets value from URL
	urlIDs := r.URL.Query()["race_id"]
	if len(urlIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": "missing race id"})
		return
	}

	//This endpoint will only accept 1 race, so throw out the rest
	raceID, err := strconv.Atoi(urlIDs[0])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	selectRace(raceID)
}

func disconnectFromTwitchChatHandler(w http.ResponseWriter, r *http.Request) {
	disconnectFromTwitchChat()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}

/*
* POST
* Takes url-encoded command string in URL and race_id
* Command string is keyed as "command"
*/
func parseCommandHandler(w http.ResponseWriter, r*http.Request) {
	//Gets values from URL
	urlCommandStr := r.URL.Query()["command"]
	if len(urlCommandStr) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": "empty command"})
		return
	}

	urlIDs := r.URL.Query()["race_id"]
	if len(urlIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": "missing race id"})
		return
	}

	//This endpoint will only accept 1 race, so throw out the rest
	raceID, err := strconv.Atoi(urlIDs[0])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	//Only accept 1 command at a time
	command := urlCommandStr[0]

	//Export command
	err = handleCommand(raceID, command)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}

/*
* POST
* Takes race_id to start in URL
*/
func startRaceHandler(w http.ResponseWriter, r *http.Request) {
	err := startRace()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}

/*
* POST
* Takes race_id to start in URL
*/
func finishRaceHandler(w http.ResponseWriter, r *http.Request) {
	err := finishRace()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}

/*
* POST
* Takes race_id to start in URL
*/
func resetRaceHandler(w http.ResponseWriter, r *http.Request) {
	err := resetRace()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}