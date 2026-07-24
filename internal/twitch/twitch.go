package twitch

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"

	"github.com/multimario_client/internal/twitch/auth"
)

//General functions for interacting with twitch API

type TwitchCredentials struct {
	userToken string
	clientID string
	clientSecret string
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

func SetTwitchParams(userToken string, clientID string, clientSecret string) {
	defaultCredentials.userToken = userToken
	defaultCredentials.clientID = clientID
	defaultCredentials.clientSecret = clientSecret
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

func (c *TwitchCredentials) ClientSecret() string {
	return c.clientSecret
}

func (c *TwitchCredentials) SetNewUserToken(newToken string) {
	c.userToken =  newToken
}

//Takes twitch user token and returns the login name for that account
func GetUserNameFromToken(userToken string, clientID string) (string, error) {
	resp, err := doGetTwitchUsersRequest("https://api.twitch.tv/helix/users", userToken, clientID)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	//If unauthorized, attempt to refresh
	if resp.StatusCode == http.StatusUnauthorized {
		newToken, err := auth.RefreshExpiredToken(defaultCredentials.clientID, defaultCredentials.clientSecret)
		if err != nil {
			//Failed refresh, return error
			return "", errors.New("unable to refresh twitch user token. try resetting the bot.")
		}

		//New token is valid. Save it and try the request again
		SetTwitchParams(newToken, clientID, defaultCredentials.clientSecret)
		resp.Body.Close() //Close current request and send new one

		resp, err = doGetTwitchUsersRequest("https://api.twitch.tv/helix/users", newToken, clientID)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		//Still failed, return error
		if resp.StatusCode != http.StatusOK {
			return "", errors.New("unable to refresh user token. try resetting the bot.")
		}
	}

	//Other failure status codes
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

		//Get response
		resp, err := doGetTwitchUsersRequest(reqURL.String(), defaultCredentials.userToken, defaultCredentials.clientID)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		//If we're unauthorized, try to get a refresh token
		if resp.StatusCode == http.StatusUnauthorized {
			newToken, err := auth.RefreshExpiredToken(defaultCredentials.clientID, defaultCredentials.clientSecret)
			if err != nil {
				//Failed refresh, return error
				return nil, errors.New("unable to refresh twitch user token. try resetting the bot.")
			}

			//New token is valid. Save it and try the request again
			SetTwitchParams(newToken, defaultCredentials.clientID, defaultCredentials.clientSecret)
			resp.Body.Close() //Close current request and send new one

			resp, err = doGetTwitchUsersRequest(reqURL.String(), newToken, defaultCredentials.clientID)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			//Still failed, return error
			if resp.StatusCode != http.StatusOK {
				return nil, errors.New("unable to refresh user token. try resetting the bot.")
			}
		}

		//Check for other errors
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

		resp.Body.Close() //Close just to not stack these deferred calls. Might as well since this is just a no-op if the body is already closed
	}

	return out, nil
}

//Helper for setting up a request to twitch users endpoint
func doGetTwitchUsersRequest(uri string, userToken string, clientID string) (*http.Response, error){
	//Create request
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	authHeader := fmt.Sprintf("Bearer %s", userToken)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Client-Id", clientID)

	//Get response
	twitchClient := http.Client{}
	resp, err := twitchClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}