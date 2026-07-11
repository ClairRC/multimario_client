package twitch

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
)

//General functions for interacting with twitch API

type TwitchCredentials struct {
	userToken string
	clientID string
}

type TwitchUserResponse struct {
	Data []TwitchUser `json:"data"`
}

type TwitchUser struct {
	ID string `json:"id"`
	Login string `json:"login"`
	DisplayName string `json:"display_name"`
	ProfilePictureURL string `json:"profile_image_url"`
}

var defaultCredentials = &TwitchCredentials{}

func SetTwitchParams(userToken string, clientID string) {
	defaultCredentials.userToken = userToken
	defaultCredentials.clientID = clientID
}

func GetTwitchParams() *TwitchCredentials {
	return defaultCredentials
}

func (c *TwitchCredentials) UserToken() string {
	return c.userToken
}

func (c *TwitchCredentials) ClientID() string {
	return c.clientID
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

//Calls twitch API for user info for things such as player name and profile image URL
func GetTwitchInfoFromUserNames(userNames []string) ([]TwitchUser, error) {
	if len(userNames) == 0 {
		return make([]TwitchUser, 0), nil
	}

	twitchClient := http.Client{}

	//Twitch doesn't allow more than 100 users per request, so batch this
	out := make([]TwitchUser, 0)
	numBatches := math.Ceil(float64(len(userNames))/100.0)

	for i := 0; i < int(numBatches); i++ {
		startIndex := i * 100
		endIndex := math.Min(float64(startIndex+100), float64(len(userNames)))

		//Create request
		reqURL, err := url.Parse("https://api.twitch.tv/helix/users")
		if err != nil {
			return nil, err
		}

		query := reqURL.Query()
		for j := startIndex; j < int(endIndex); j++ {
			query.Add("login", userNames[j])
		}
		reqURL.RawQuery = query.Encode()

		req, err := http.NewRequest("GET", reqURL.String(), nil)
		if err != nil {
			return nil, err
		}

		//Auth
		authHeader := fmt.Sprintf("Bearer %s", defaultCredentials.userToken)
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Client-Id", defaultCredentials.clientID)

		//Get response
		resp, err := twitchClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		//Check for errors
		//TODO: Add a retry to attempt to refresh token.
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New("unknown error getting user info from twitch: " + http.StatusText(resp.StatusCode)) //TODO: Could be more specific based on response code
		}

		//Parse the response
		var twitchUserResp TwitchUserResponse
		err = json.NewDecoder(resp.Body).Decode(&twitchUserResp)
		if err != nil {
			return nil, errors.New("unknown error parsing twitch response. could not parse as json")
		}

		//Add the responses to the output
		for _, o := range twitchUserResp.Data {
			out = append(out, o)
		}
	}

	return out, nil
}