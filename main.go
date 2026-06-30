package main

import (
	"encoding/json"
	"os"

	"github.com/multimario_client/internal/controlpanel"
)

const settingsPath = "settings.json"

type Settings struct {
	TwitchClientID string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
}

func main() {
	/*
	//Load settings
	settings, err := loadSettings(settingsPath)
	if err != nil {
		log.Fatalf("unable to load twitch api information from %s", settingsPath)
	}

	//Get twitch user token
	token, err := auth.GetUserToken(settings.TwitchClientID, settings.TwitchClientSecret)
	if err != nil {
		log.Fatalf("%v", err)
	}

	//Connect to twitch chat
	err = chat.ConnectToChat(token, settings.TwitchClientID, client.DefaultMMClient.GetRacerTwitchNames())
	if err != nil {
		fmt.Printf("%v", err)
	}
	*/

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