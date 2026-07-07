package controlpanel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/stats"
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

func sendUpcomingRaces(w http.ResponseWriter, r *http.Request) {
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

func sendCompletedRaces(w http.ResponseWriter, r *http.Request) {
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
func sendInProgressRace(w http.ResponseWriter, r *http.Request) {
	//Get in progress race
	w.Header().Set("Content-Type", "application/json")

	race, err := mmapi.GetInProgressRace()
	if err != nil {
		logMessage(fmt.Sprintf("ERROR: %s", err.Error()))
		return
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
func isConnectedToTwitch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"connected": chat.Client.IsConnectedToTwitch()})
}

/*
* POST
* Takes race_id to start in URL
*/
func connectToTwitchChat(w http.ResponseWriter, r *http.Request) {
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

	//After we have the raceID, get a list of twitch channels from the backend
	playerRecords, err := mmapi.GetPlayersForRace(raceID)

	//We only need the channel names, so convert them into a string array 
	twitchChannels := make([]string, 0)
	for _, record := range playerRecords {
		twitchChannels = append(twitchChannels, record.PlayerTwitch)
	}

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	//Connect to chat
	chat.Client.ConnectToChat(twitchChannels, logC)
}

/*
* POST
* Takes race_id in URL
*/

//Function that takes race information and then passes that along to the stats stream
func selectRace(w http.ResponseWriter, r *http.Request) {
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

	//Start race on stats stream
	err = stats.StartTrackingRace(raceID, "00:00:00", false)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errMsg := fmt.Sprintf("unable to begin tracking race: %s", err.Error())
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": errMsg})
		return
	}
}

func disconnectFromTwitchChat(w http.ResponseWriter, r *http.Request) {
	chat.Client.DisconnectFromChat(logC)
}

/*
* POST
* Takes race_id to start in URL
*/
func startRace(w http.ResponseWriter, r *http.Request) {
	//Set new status to in_progress and pass off to helper function
	updateRaceStatus(w, r, "in_progress")
}

/*
* POST
* Takes race_id to start in URL
*/
func finishRace(w http.ResponseWriter, r *http.Request) {
	//Set new status to completed
	updateRaceStatus(w, r, "completed")
}

/*
* POST
* Takes race_id to start in URL
*/
func resetRace(w http.ResponseWriter, r *http.Request) {
	//Set new status to upcoming
	updateRaceStatus(w, r, "upcoming")
}

//Helper for updating race status since multiple endpoints will do this
func updateRaceStatus(w http.ResponseWriter, r *http.Request, newStatus string) {
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

	//Start the race
	err = mmapi.UpdateRaceStatus(raceID, newStatus)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any {"success": false, "error": err.Error()})
		return
	}

	//No errors
	logMessage(fmt.Sprintf("Race %v status has been updated to \"%s\"", raceID, newStatus))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any {"success": true})
}