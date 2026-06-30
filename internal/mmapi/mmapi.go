package mmapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

//Package for API communication with the multimario API

//Default values for the API
var ip = "http://localhost"
var port = ":3000"
var key = ""

type RaceRes struct {
	Success bool `json:"success"`
	Error string `json:"error"`
	ID int `json:"id"`
	Races []RaceInfo `json:"races"`
}

type RaceInfo struct {
	Category string `json:"category"`
	Date string `json:"date"`
	ID float64 `json:"id"`
	Status string `json:"status"`
}

//Function to set up mm api information
func SetMultiMarioAPIParams(apiIP string, apiPort string, apiKey string) {
	ip = apiIP
	port = apiPort
	key = apiKey
}

func GetUpcomingRaces() ([]RaceInfo, error) {
	return getRacesFromStatus("upcoming")
}

func GetCompletedRaces() ([]RaceInfo, error) {
	return getRacesFromStatus("completed")
}

//Helper for getting races based on status
func getRacesFromStatus(status string) ([]RaceInfo, error) {
	//Get upcoming races from mm api
	endpoint := fmt.Sprintf("%s%s/races?status=%s", ip, port, status)
	req, err := http.NewRequest("GET", endpoint , nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{}

	//Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	//Parse response into JSON
	raceResponse := RaceRes{}
	json.NewDecoder(resp.Body).Decode(&raceResponse)

	if !raceResponse.Success {
		return nil, errors.New(raceResponse.Error)
	}

	return raceResponse.Races, nil
}

func GetInProgressRace() (*RaceInfo, error) {
	//Get upcoming races from mm api
	req, err := http.NewRequest("GET", ip+port+"/races?status=in_progress", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{}

	//Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	//Parse response into JSON
	raceResponse := RaceRes{}
	json.NewDecoder(resp.Body).Decode(&raceResponse)

	if !raceResponse.Success {
		return nil, errors.New(raceResponse.Error)
	}

	if len(raceResponse.Races) == 0 {
		return nil, nil
	}

	return &raceResponse.Races[0], nil
}

//Updates a given Race status. Returns error if there is one
func UpdateRaceStatus(raceID int, newStatus string) error {
	//Get Request body
	body := map[string]any {
		"status": newStatus,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	//Get Request
	req, err := http.NewRequest("PATCH", ip+port+"/races/"+strconv.Itoa(raceID), bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	//Send request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	//Check response
	var respMap map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		return err
	}

	//Parse response
	success, ok := respMap["success"].(bool)
	if !ok {
		return errors.New("unable to parse api response")
	}

	if !success {
		apiErr, ok := respMap["error"].(string)
		if !ok {
			apiErr = "unknown failure with api response"
		}
		return errors.New(apiErr)
	}

	return nil
}

//Updates race start time. Must be formatted as hh:mm:ss
func UpdateRaceStartTime(raceID int, newStartTime string) error {
	//Get Request body
	body := map[string]any {
		"start_time": newStartTime,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	//Get Request
	req, err := http.NewRequest("PATCH", ip+port+"/races/"+strconv.Itoa(raceID), bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	//Send request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	//Check response
	var respMap map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		return err
	}

	//Parse response
	success, ok := respMap["success"].(bool)
	if !ok {
		return errors.New("unable to parse api response")
	}

	if !success {
		apiErr, ok := respMap["error"].(string)
		if !ok {
			apiErr = "unknown failure with api response"
		}
		return errors.New(apiErr)
	}

	return nil
}