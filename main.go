package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/multimario_client/internal/controlpanel"
	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/stats"
	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch"
	"github.com/multimario_client/internal/twitch/auth"
	"github.com/multimario_client/internal/twitch/chat"
)

const settingsPath = "settings.json"

type Settings struct {
	TwitchClientID string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
	MMAPIKey string `json:"multimario_api_key"`
	Layout string `json:"layout"`
}

func main() {
	//Load settings
	settings, err := loadSettings(settingsPath)
	if err != nil {
		log.Fatalf("unable to load twitch api information from %s: %s", settingsPath, err.Error())
	}
	mmapi.SetMultiMarioAPIParams("http://localhost", ":3000", settings.MMAPIKey)

	//Get twitch user token
	token, err := auth.GetUserToken(settings.TwitchClientID, settings.TwitchClientSecret)
	if err != nil {
		log.Fatalf("%v", err)
	}

	//Set twitch parameters
	twitch.SetTwitchParams(token, settings.TwitchClientID, settings.TwitchClientSecret)
	chat.Client.SetTwitchConnectionParams(twitch.GetTwitchParams())

	//Check if there's an in progress race and if so store that
	race, err := mmapi.GetInProgressRace()
	if err != nil {
		log.Fatalf("%v", err)
	}

	//Store race if it exists
	if race != nil {
		store.Race.LoadRace(int(race.ID))
	}

	//Initialize control panel
	go controlpanel.InitControlPanel()
	stats.InitStatsPage(settings.Layout)
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