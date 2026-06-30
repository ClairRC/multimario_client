package twitch

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

//General functions for interacting with twitch API

type TwitchUserResponse struct {
	Data []TwitchUser `json:"data"`
}

type TwitchUser struct {
	ID string `json:"id"`
	Login string `json:"login"`
}

//Takes twitch user token and returns the login name for that account
func GetUserNameFromToken(userToken string, clientID string) (string, error) {
	//Create request
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return "", err
	}

	authHeader := fmt.Sprintf("Bearer %s", userToken)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Client-Id", clientID)

	//Get response
	twitchClient := http.Client{}
	resp, err := twitchClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	//Check for errors
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("unknown error getting user info from twitch: " + http.StatusText(resp.StatusCode)) //TODO: Could be more specific based on response code
	}

	//Parse the response
	var twitchUserResp TwitchUserResponse
	err = json.NewDecoder(resp.Body).Decode(&twitchUserResp)
	if err != nil {
		return "", errors.New("unknown error parsing twitch response. could not parse as json")
	}

	if len(twitchUserResp.Data) == 0 {
		return "", errors.New("unable to get user token user")
	}

	return twitchUserResp.Data[0].Login, nil
}