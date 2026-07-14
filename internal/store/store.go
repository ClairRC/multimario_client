package store

import (
	"errors"
	"fmt"
	"maps"
	"sync"

	categoryinfo "github.com/multimario_client/internal/category_info"
	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/stats"
	"github.com/multimario_client/internal/twitch"
)

//This package is a layer that handles orchestration between commands, local caching, and pushing updates to the stats stream.

//For caching information about current race state
type Store struct {
	mu sync.RWMutex
	raceID int
	players map[string]*stats.PlayerInfo
	info *mmapi.RaceInfo
	organizers map[string]bool
	timerRunning bool
}

var Race = &Store{raceID: -1, players: nil, info: nil}
var noRaceLoadedErr error = errors.New("no race is currently loaded")

var organizerMu sync.RWMutex
var organizerList map[string]bool = map[string]bool{"clairdss": true}

var blacklistMu sync.RWMutex
var blacklist map[string]bool

func (s *Store) LoadRace(raceID int) error {
	//Get race info from the backend
	raceInfo, err := mmapi.GetRaceFromID(raceID)
	if err != nil {
		return err
	}

	//Get player info from this race
	players, err := mmapi.GetPlayersForRace(raceID)
	if err != nil {
		return err
	}

	//Use these records to get necessary twitch information
	playerNames := make([]string, 0) //Slice to get twitch info
	playerNameMap := make(map[string]*stats.PlayerInfo) //Map to append new twitch info
	for _, p := range players {
		playerNames = append(playerNames, p.PlayerTwitch)
		//Add incomplete playerInfo to the map
		playerNameMap[p.PlayerTwitch] = &stats.PlayerInfo{
			NumCollected: p.NumCollected,
			Player: p.Player,
			PlayerTwitch: p.PlayerTwitch,
			FinalTime: p.FinalTime,
			Estimate: p.Estimate,
			Status: "running",
			ProfilePictureURL: "",
		}
	}
	twitchUsers, err := twitch.GetTwitchInfoFromUserNames(playerNames)
	if err != nil {
		return err
	}

	//Fill in the blanks
	for _, u := range twitchUsers {
		pInfo := playerNameMap[u.Login]
		pInfo.ProfilePictureURL = u.ProfilePictureURL 
	}

	//Add each value into a slice
	recordsSlice := make([]stats.PlayerInfo, 0)
	for v := range maps.Values(playerNameMap) {
		recordsSlice = append(recordsSlice, *v)
	}

	//Saves information for this race in the current store
	s.mu.Lock()
	s.raceID = raceID
	s.players = playerNameMap
	s.info = raceInfo
	s.mu.Unlock()

	//Loads race information to begin tracking
	err = stats.StartTrackingRace(raceID, recordsSlice, raceInfo.Category, "00:00:00", true)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) GetRacerTwitchChannels() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return nil, noRaceLoadedErr
	}

	out := make([]string, 0)
	for p := range maps.Values(s.players) {
		out = append(out, p.PlayerTwitch)
	}

	return out, nil
}

//Adds to player's count. Returns new number of collectibles
func (s *Store) AddToPlayerCount(numToAdd int, playerTwitch string) (int, error) {
    s.mu.RLock()
    if !s.raceIsLoaded() {
        s.mu.RUnlock()
        return -1, noRaceLoadedErr
    }
    p, ok := s.players[playerTwitch]
    raceID := s.raceID
    s.mu.RUnlock()
    if !ok {
        return -1, errors.New("player is not in this race")
    }

    newNum, err := mmapi.IncrementPlayerCount(raceID, p.Player, numToAdd)
    if err != nil {
        return -1, err
    }

    s.mu.Lock()
    p.NumCollected = float64(newNum)
    s.mu.Unlock()

    if err := stats.UpdatePlayerCount(p.PlayerTwitch, newNum); err != nil {
        return -1, err
    }

    return newNum, nil
}

func (s *Store) SetPlayerCount(numToSet int, playerTwitch string) (int, error) {
	s.mu.RLock()
    if !s.raceIsLoaded() {
        s.mu.RUnlock()
        return -1, noRaceLoadedErr
    }
    p, ok := s.players[playerTwitch]
    raceID := s.raceID
    s.mu.RUnlock()
    if !ok {
        return -1, errors.New("player is not in this race")
    }

    newNum, err := mmapi.SetPlayerCount(raceID, p.Player, numToSet)
    if err != nil {
        return -1, err
    }

    s.mu.Lock()
    p.NumCollected = float64(newNum)
    s.mu.Unlock()

    if err := stats.UpdatePlayerCount(p.PlayerTwitch, newNum); err != nil {
        return -1, err
    }

    return newNum, nil
}

//Gets actual player name from their Twitch
func (s *Store) GetPlayerNameFromTwitch(playerTwitchName string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return "", noRaceLoadedErr
	}

	if s.players[playerTwitchName] == nil {
		return "", errors.New("player not in this race")
	}

	return s.players[playerTwitchName].Player, nil
}

func (s *Store) UpdatePlayerName(playerTwitchName string, newName string) error {
	//Get player name. Use their display name if possible
	//This isn't necessary but it's faster for the API, but that's another can of worms
	currentName := playerTwitchName
	playerInLoadedRace := false

	s.mu.RLock()
	if s.raceIsLoaded() {
		player, ok := s.players[playerTwitchName]
		if ok {
			currentName = player.Player
			playerInLoadedRace = true
		}
	}
	s.mu.RUnlock()

	//Send the update to the backend
	err := mmapi.UpdatePlayerName(currentName, newName)
	if err != nil {
		return err
	}

	//If player is in race, update it
	if playerInLoadedRace {
		s.mu.Lock()
		if p, ok := s.players[playerTwitchName]; ok {
			p.Player = newName
		}
		s.mu.Unlock()
	}

	//Update stats stream
	err = stats.UpdatePlayerName(playerTwitchName, newName)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) UpdateGameTime(playerTwitchName string, gameName string, newTime string) error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}

	//Get player name. Use their display name if possible
	//This isn't necessary but it's faster for the API, but that's another can of worms
	currentName := playerTwitchName

	if s.raceIsLoaded() {
		player, ok := s.players[playerTwitchName]
		if ok {
			currentName = player.Player
		}
	}

	//Get the game category
	gameCatName := categoryinfo.GetGameCategoryFromGameName(s.info.Category, gameName)
	raceID := s.raceID
	s.mu.RUnlock()

	//Send the update to the backend
	err := mmapi.UpdatePlayerCategoryTime(raceID, currentName, gameCatName, newTime)
	
	return err
}

func (s *Store) UpdateFinalTime(playerTwitchName string, newTime string) error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}

	//Get player name. Use their display name if possible
	//This isn't necessary but it's faster for the API, but that's another can of worms
	currentName := playerTwitchName
	playerInLoadedRace := false

	if s.raceIsLoaded() {
		player, ok := s.players[playerTwitchName]
		if ok {
			currentName = player.Player
			playerInLoadedRace = true
		}
	}
	raceID := s.raceID
	s.mu.RUnlock()

	//Send the update to the backend
	err := mmapi.UpdatePlayerFinalTime(raceID, currentName, newTime)
	if err != nil {
		return err
	}
	
	//Update player's time and send it to the stats page if they are in the loaded race
	if playerInLoadedRace {
		s.mu.Lock()
		if p, ok := s.players[playerTwitchName]; ok {
			p.FinalTime = newTime
		}
		s.mu.Unlock()

		err = stats.UpdatePlayerFinalTime(playerTwitchName, newTime)
		if err != nil {
			return err
		}
	}

	return nil
}

//Sets player's status to quit
func (s *Store) SetToQuit(playerTwitchName string) error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.players[playerTwitchName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%s is not in this race", playerTwitchName)
	}

	s.mu.Lock()
	p.Status = "Forfeit"
	s.mu.Unlock()

	//Get the lock again
	s.mu.RLock()
	p, ok = s.players[playerTwitchName]
	newStatus := p.Status

	stats.SetPlayerStatus(playerTwitchName, newStatus)
	s.mu.RUnlock()

	return nil
}

//Sets player's status to not quit (running)
func (s *Store) SetToRunning(playerTwitchName string) error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.players[playerTwitchName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%s is not in this race", playerTwitchName)
	}

	s.mu.Lock()
	p.Status = "running"
	s.mu.Unlock()

	//Get the lock again
	s.mu.RLock()
	p, ok = s.players[playerTwitchName]
	newStatus := p.Status

	stats.SetPlayerStatus(playerTwitchName, newStatus)
	s.mu.RUnlock()

	return nil
}

//Sets player's status to custom value
func (s *Store) SetPlayerStatus(playerTwitchName string, newStatus string) error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.players[playerTwitchName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%s is not in this race", playerTwitchName)
	}

	s.mu.Lock()
	p.Status = newStatus
	s.mu.Unlock()


	//Get the lock again
	s.mu.RLock()
	p, ok = s.players[playerTwitchName]
	confirmedNewStatus := p.Status

	stats.SetPlayerStatus(playerTwitchName, confirmedNewStatus)
	s.mu.RUnlock()

	return nil
}

//Sets player's status to custom value
func (s *Store) SetTimerValue(newTimerValue string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	err := stats.UpdateTimerValue(newTimerValue)
	if err != nil {
		return err
	}

	return nil
}

//Stops the timer
func (s *Store) StopTimer() error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}
	s.mu.RUnlock()

	//Set the current value to be started
	s.mu.Lock()
	s.timerRunning = false
	stats.UpdateTimerState(false)
	s.mu.Unlock()

	return nil
}

//Starts the timer
func (s *Store) StartTimer() error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}
	s.mu.RUnlock()

	//Set the current value to be started
	s.mu.Lock()
	s.timerRunning = true
	stats.UpdateTimerState(true)
	s.mu.Unlock()

	return nil
}

//Adds a user as an organizer
func AddOrganizer(newOrganizer string) {
	organizerMu.Lock()
	organizerList[newOrganizer] = true
	organizerMu.Unlock()
}


//Checks if a player is an organizer by twitch name
func IsOrganizer(playerTwitchName string) bool {
	organizerMu.RLock()
	defer organizerMu.RUnlock()

	return organizerList[playerTwitchName]
}

//Adds a user to blacklist
func AddBlacklistUser(user string) {
	blacklistMu.Lock()
	blacklist[user] = true
	organizerMu.Unlock()
}

//Removes a user from blacklist
func RemoveBlacklistUser(user string) {
	blacklistMu.Lock()
	blacklist[user] = false
	organizerMu.Unlock()
}


//Checks if a player is an organizer by twitch name
func IsOnBlacklist(user string) bool {
	blacklistMu.RLock()
	defer blacklistMu.RUnlock()

	return blacklist[user]
}

//Getter for getting current race category
func (s *Store) GetCurrentRaceCategory() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return "", noRaceLoadedErr
	}
	return s.info.Category, nil
}


//Helper to check that a race is loaded
func (s *Store) raceIsLoaded() bool {
	if s.raceID == -1 || s.players == nil {
		return false
	}

	return true
}
