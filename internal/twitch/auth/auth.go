package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"
)

type twitchUserToken struct {
	token string
	refreshToken string
}

const userTokenPath = "auth.json"

//Twitch callback server information
var port string = ":3000"
var ip string = "http://localhost"

//Gets user token
func GetUserToken(clientID string, clientSecret string) (string, error) {
	//Try to get saved token first
	userToken, err := getSavedToken(userTokenPath)
	if err == nil {
		//After getting this token, validate it
		tokenIsValid, err := userTokenIsValid(userToken.token)
		if err != nil {
			return "", err
		}

		//Token is valid, just return it
		if tokenIsValid {
			return userToken.token, nil	
		}

		//Try to refresh the token
		userToken, err = refreshToken(userToken, clientID, clientSecret)
		if err == nil {
			//No errors, token has been refresh
			saveUserToken(userTokenPath, userToken)
			return userToken.token, nil
		}
	}

	//Unable to get token, get Twitch authorization
	authURI := getUserAuthURI(clientID)
	
	//Set up local server for Twitch callback
	errC := make(chan error, 1)
	codeC := make(chan string, 1)

	//Open the server and listen for Twitch callback
	mux := http.NewServeMux()
	server := http.Server{Addr: port, Handler: mux}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		//Write error description if error is not empty
		if r.URL.Query().Get("error") != "" {
			w.Write([]byte("no code in callback"))
			errC <- fmt.Errorf("error getting code for access token: %s", r.URL.Query().Get("error_description"))
			return
		}

		//Success
		w.Write([]byte("Authentication successful: You can close this tab."))
		codeC <- r.URL.Query().Get("code")
	})
	go server.ListenAndServe()
	browser.OpenURL(authURI)

	//Handle callback results
	select {
	case code := <-codeC:
		server.Shutdown(context.Background())
		userToken, err := exchangeCode(clientID, clientSecret, code)
		if err != nil {
			return "", err
		}
		saveUserToken(userTokenPath, userToken)
		return userToken.token, nil
	case err := <-errC:
		server.Shutdown(context.Background())
		return "", err
	case <-time.After(30 * time.Second):
		server.Shutdown(context.Background())
		return "", fmt.Errorf("auth request timed out")
	}

}

//Attempt to refresh expired user token
func RefreshExpiredToken(clientID string, clientSecret string) (string, error) {
	//Get the current token
	oldToken, err := getSavedToken(userTokenPath)
	if err != nil {
		return "", err
	}

	//Refresh the token
	newToken, err := refreshToken(oldToken, clientID, clientSecret)
	if err != nil {
		return "", err
	}

	//No errors, save the token
	saveUserToken(userTokenPath, newToken)

	return newToken.token, nil
}

//Exchanges Twitch auth code for user token
func exchangeCode(clientID string, clientSecret string, code string) (*twitchUserToken, error) {
	//Create post request for access token
	uri := "https://id.twitch.tv/oauth2/token"
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("client_secret", clientSecret)
	v.Set("code", code)
	v.Set("grant_type", "authorization_code")
	v.Set("redirect_uri", ip+port+"/callback")

	//Send post request
	res, err := http.Post(uri, "application/x-www-form-urlencoded", strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	//Check response code
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting token from twitch: %s", res.Status)
	}

	//Encode body as map
	var resMap map[string]any
	err = json.NewDecoder(res.Body).Decode(&resMap)
	if err != nil{
		return nil, err
	}

	var newToken twitchUserToken
	token, ok := resMap["access_token"].(string)
	if !ok {
		return nil, fmt.Errorf("unknown error parsing twitch response")
	}
	refresh, ok := resMap["refresh_token"].(string)
	if !ok {
		return nil, fmt.Errorf("unknown error parsing twitch response")
	}
	newToken.token = token
	newToken.refreshToken = refresh

	return &newToken, nil
}

//Gets saved user token from auth JSON
func getSavedToken(tokenJSONPath string) (*twitchUserToken, error){
	tokenFile, err := os.Open(tokenJSONPath)
	if err != nil {
		return nil, err
	}
	defer tokenFile.Close()

	tokenMap := make(map[string]any)
	err = json.NewDecoder(tokenFile).Decode(&tokenMap)
	if err != nil {
		return nil, err
	}

	token, ok := tokenMap["token"].(string)
	//Unable to get saved token for some reason
	if !ok || token == "" {
		return nil, errors.New("unable to parse token from token auth file")
	}
	refreshToken, ok := tokenMap["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, errors.New("unable to parse refresh token from token auth file")
	}

	return &twitchUserToken{token, refreshToken}, nil
}

//Saves user token to JSON file
func saveUserToken(tokenJSONPath string, userToken *twitchUserToken) {
	//R/W only for owner
	tokenFile, err := os.OpenFile(tokenJSONPath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer tokenFile.Close()

	rawData := make(map[string]any)
	rawData["token"] = userToken.token
	rawData["refresh_token"] = userToken.refreshToken

	//Write token
	json.NewEncoder(tokenFile).Encode(rawData)
}

//Takes a user token, refreshes it, and then returns a pointer to the new token
//Takes an optional callback to call once the token has been refreshed
func refreshToken(userToken *twitchUserToken, clientID string, clientSecret string) (*twitchUserToken, error) {
	//Get request
	refreshEndpoint := "https://id.twitch.tv/oauth2/token"
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("client_secret", clientSecret)
	v.Set("grant_type", "refresh_token")
	v.Set("refresh_token", userToken.refreshToken)

	req, err := http.NewRequest("POST", refreshEndpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	//Send request
	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("unable to refresh twitch user token")
	}

	//Get new values from response
	var resMap map[string]any
	err = json.NewDecoder(res.Body).Decode(&resMap)
	if err != nil {
		return nil, err
	}

	newToken, ok := resMap["access_token"].(string)
	if !ok {
		return nil, errors.New("unable to parse new user token")
	}

	newRefresh, ok := resMap["refresh_token"].(string)
	if !ok {
		return nil, errors.New("unable to parse new refresh token")
	}

	out := &twitchUserToken{token: newToken, refreshToken: newRefresh}
	return out, nil
}

//Checks that user token is valid
func userTokenIsValid(token string) (bool, error) {
	//Send request to Twitch
	validateEndpoint := "https://id.twitch.tv/oauth2/validate"
	req, err := http.NewRequest("GET", validateEndpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("OAuth %s", token))
	client := http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == http.StatusOK, nil
} 

func getUserAuthURI(clientID string) string {
	uri := "https://id.twitch.tv/oauth2/authorize?"
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("redirect_uri", ip + port + "/callback")
	v.Set("response_type", "code")
	v.Set("scope", "chat:read chat:edit")
	v.Set("force_verify", "true")

	return uri + v.Encode()
}