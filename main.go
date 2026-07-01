package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/multimario_client/internal/controlpanel"
	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/twitch/auth"
	"github.com/multimario_client/internal/twitch/chat"
)

const settingsPath = "settings.json"

type Settings struct {
	TwitchClientID string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
	MMAPIKey string `json:"multimario_api_key"`
}

func main() {
	//Load settings
	settings, err := loadSettings(settingsPath)
	if err != nil {
		log.Fatalf("unable to load twitch api information from %s", settingsPath)
	}
	mmapi.SetMultiMarioAPIParams("http://localhost", ":3000", settings.MMAPIKey)

	//Get twitch user token
	token, err := auth.GetUserToken(settings.TwitchClientID, settings.TwitchClientSecret)
	if err != nil {
		log.Fatalf("%v", err)
	}

	//Set twitch parameters
	chat.Client.SetTwitchConnectionParams(token, settings.TwitchClientID)

	//Initialize control panel
	controlpanel.InitControlPanel()
}

func loadSettings(settingsPath string) (*Settings, error) {
	//Load settings
	settingsFile, err := os.Open(settingsPath)
	if err != nil {
		return nil, err
	}
	defer settingsFile.Close()

	var settings Settings
	err = json.NewDecoder(settingsFile).Decode(&settings)
	if err != nil {
		return nil, err
	}

	return &settings, nil
}