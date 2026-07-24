package controlpanel

import (
	"fmt"

	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch/chat"
)

//Helper functions for interacting with race state

//Selects race
func selectRace(raceID int) error {
	//Start race on stats stream
	err := store.Race.LoadRace(raceID)
	if err != nil {
		return err
	}

	return nil
}

//Connects to twitch chat of the loaded race
func connectToTwitchChat() error {
	//Connect to chat
	twitchChannels, err := store.Race.GetRacerTwitchChannels()
	if err != nil {
		return err
	}

	return chat.Client.ConnectToChat(twitchChannels, logMessage)
}

//Disconnects from twitch
func disconnectFromTwitchChat() {
	chat.Client.DisconnectFromChat(logMessage)
}

//Starts loaded race
func startRace() error {
	//Set new status to in_progress and pass off to helper function
	err := store.Race.SetTimerValue("00:00:00")
	if err != nil {
		return err
	}

	err = store.Race.StartTimer()
	if err != nil {
		return err
	}

	err = updateRaceStatus("in_progress")
	if err != nil {
		return err
	}

	return nil
}

func finishRace() error {
	//Set new status to completed and pass off to helper function
	err := store.Race.StopTimer()
	if err != nil {
		return err
	}

	err = updateRaceStatus("completed")
	if err != nil {
		return err
	}

	return nil
}

func resetRace() error {
	//Set new status to upcoming and pass off to helper function
	err := store.Race.StopTimer()
	if err != nil {
		return err
	}

	err = updateRaceStatus("upcoming")
	if err != nil {
		return err
	}

	return nil
}

func updateRaceStatus(newStatus string) error {
	err := store.Race.UpdateRaceStatus(newStatus)
	if err != nil {
		return err
	}

	//No error, log success
	logMessage(fmt.Sprintf("Race %v has been updated to %s", store.Race.GetCurrentRaceID(), newStatus))
	return nil
}