package controlpanel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/multimario_client/internal/mmapi"
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
func pauseRace(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(map[string]any {"error": "missing race id"})
		return
	}

	//This endpoint will only accept 1 race, so throw out the rest
	raceID, err := strconv.Atoi(urlIDs[0])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any {"error": err.Error()})
		return
	}

	//Start the race
	err = mmapi.UpdateRaceStatus(raceID, newStatus)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any {"error": err.Error()})
		return
	}

	//No errors
	w.WriteHeader(http.StatusOK)
}