package store

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	categoryinfo "github.com/multimario_client/internal/category_info"
	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/twitch"
)

//This package is a layer that handles orchestration between commands, local caching, and pushing updates to the stats stream.

//For caching information about current race state
type store struct {
	mu sync.RWMutex
	state *State
	subscribers map[int]*subscriber
	info *mmapi.RaceInfo
	next int
}

//Struct for sending full state to subscribers
type State struct {
	RaceID int
	Players map[string]*PlayerInfo
	Timer *TimerInfo
	Category string
}

//Struct for sending updates to subscribers
type Update struct {
	Type UpdateType //Tracking type of update
	Player *PlayerInfo
	Game *GameInfo
	Timer *TimerInfo
}

//Type of update metadata for subscribers
type UpdateType int
const (
	All UpdateType = iota
	PlayerCount
	PlayerName
	GameTime
	FinalTime
	PlayerStatus
	StartTime
	TimerState
)

//Struct for player info that gets sent to subscribers
type PlayerInfo struct {
	//Similar to mmapi.records, but adds some Twitch info for stats stream to use
	NumCollected float64 `json:"num_collected"`
	Player string `json:"player_name"`
	PlayerTwitch string `json:"twitch_name"`
	PlayerTwitchDisplayName string `json:"twitch_display_name"`
	FinalTime string `json:"time"`
	Estimate string `json:"estimate"`
	Status string `json:"status"`
	ProfilePictureURL string `json:"pfp_url"`
}

//Struct for game info for updates
type GameInfo struct {
	GameName string
	GameTime string
}

//Struct for timer info for updates
type TimerInfo struct {
	TimerRunning bool
	StartTime int64
}

//Subscriber information
type subscriber struct {
	id int
	updateTypes map[UpdateType]bool
	stateCh chan(State)
	updateCh chan(Update) 
}

//State values we're saving to the cache
var cacheMu sync.RWMutex
type cachedState struct {
	Players map[string]*cachedPlayerInfo `json:"players"`
	Timer *cachedTimerInfo `json:"timer_info"`
}

type cachedPlayerInfo struct {
	Status string `json:"status"`
}

type cachedTimerInfo struct {
	TimerRunning bool `json:"timer_running"`
	StartTime int64 `json:"start_time"`
}

var Race = &store{
	state: &State{RaceID: -1, Players: nil, Timer: &TimerInfo{false, time.Now().UnixMilli()}, Category: ""},
	subscribers: make(map[int]*subscriber),
	info: nil,
	next: 0,
}

var noRaceLoadedErr error = errors.New("no race is currently loaded")

var organizerMu sync.RWMutex
var organizerList map[string]bool = make(map[string]bool)

var blacklistMu sync.RWMutex
var blacklist map[string]bool = make(map[string]bool)

var whitelistMu sync.RWMutex
var whitelist map[string]bool = make(map[string]bool)

var exportedTimesMu sync.RWMutex
var logStatesMu sync.RWMutex

var cacheDir string = "./cache"
var organizersPath = "./organizers.txt"
var blacklistPath = "./blacklist.txt"
var exportedTimesPath = "./results.json"
var logStatesDir = "./race_data/"
var whitelistPath = "./whitelist.txt"

func (s *store) Subscribe(ctx context.Context, updateTypes ...UpdateType) (chan(State), chan(Update), context.CancelFunc) {
	//Channels
	stateCh := make(chan(State), 5)
	updateCh := make(chan(Update), 5)

	s.mu.Lock()

	//ID
	newID := s.next
	s.next++
	
	//Get types of updates this subscriber wants
	types := make(map[UpdateType]bool)
	if len(updateTypes) == 0 {
		types[All] = true
	} else {
		for _, t := range updateTypes {
			types[t] = true
		}
	}

	//Get cancel function
	subCtx, cancel := context.WithCancel(ctx)

	//Make subscriber and add it to list
	newSub := &subscriber{id: newID, updateTypes: types, updateCh: updateCh, stateCh: stateCh}
	s.subscribers[newSub.id] = newSub

	//Send copy of current state
	newSub.stateCh <- *s.state
	s.mu.Unlock()

	//Check for cancellation
	go func() {
		<-subCtx.Done()
		s.unsubscribe(newSub.id)
	}()

	return newSub.stateCh, newSub.updateCh, cancel
}

func (s *store) unsubscribe(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subscribers[id]; ok {
		close(sub.stateCh)
		close(sub.updateCh)
		delete(s.subscribers, id)
	}
}

func (s *store) LoadRace(raceID int) error {
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

	//Load organizers, blacklist, whitelist
	s.loadBlacklist(blacklistPath)
	s.loadOrganizerList(organizersPath)

	//Use these records to get necessary twitch information
	playerNames := make([]string, 0) //Slice to get twitch info
	playerNameMap := make(map[string]*PlayerInfo) //Map to append new twitch info
	for _, p := range players {
		playerNames = append(playerNames, p.PlayerTwitch)
		//Add incomplete playerInfo to the map
		playerNameMap[p.PlayerTwitch] = &PlayerInfo{
			NumCollected: p.NumCollected,
			Player: p.Player,
			PlayerTwitch: p.PlayerTwitch,
			PlayerTwitchDisplayName: "",
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
		if pInfo != nil {
			pInfo.ProfilePictureURL = u.ProfilePictureURL 
			pInfo.PlayerTwitchDisplayName = u.DisplayName
		}
	}

	//Saves information for this race in the current store
	newState := &State{
		RaceID: raceID,
		Players: playerNameMap,
		Timer: &TimerInfo{false, time.Now().UnixMilli()},
		Category: raceInfo.Category,
	}
	
	//Get cached state to get possibly stale values
	cachedState := getCachedState(raceID, cacheDir)
	for player, pInfo := range newState.Players {
		//Get cached status for this player
		cachedPInfo, exists := cachedState.Players[player]
		if exists {
			pInfo.Status = cachedPInfo.Status
		}
	}

	//Update timer info
	if cachedState.Timer != nil {
		newState.Timer.StartTime = cachedState.Timer.StartTime
		newState.Timer.TimerRunning = cachedState.Timer.TimerRunning
	}

	s.mu.Lock()
	s.state = newState
	s.info = raceInfo

	//Send new state to subscribers
	for _, sub := range s.subscribers {
		select {
			case sub.stateCh <- *newState:
			default:
		}
	}
	s.mu.Unlock()

	//Do this at the end because the organizer list, blacklist, and player list must be loaded
	//And also this function locks s.mu, so this needs to be after that is released
	s.loadWhitelist(whitelistPath)

	return nil
}

func (s *store) GetRacerTwitchChannels() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return nil, noRaceLoadedErr
	}

	out := make([]string, 0)
	for p := range maps.Values(s.state.Players) {
		out = append(out, p.PlayerTwitch)
	}

	return out, nil
}

//Adds to player's count. Returns new number of collectibles
func (s *store) AddToPlayerCount(numToAdd int, playerTwitch string) (int, error) {
	//Make sure player is lowercase
	playerTwitch = strings.ToLower(playerTwitch)

    s.mu.RLock()
    if !s.raceIsLoaded() {
        s.mu.RUnlock()
        return -1, noRaceLoadedErr
    }
    p, ok := s.state.Players[playerTwitch]
    raceID := s.state.RaceID
    s.mu.RUnlock()
    if !ok {
        return -1, errors.New("player is not in this race")
    }

	//Only allow this if player is still running in the race
	if p.Status != "running" {
		return -1, errors.New("can't update player count for player who is no longer in the race")
	}

    newNum, err := mmapi.IncrementPlayerCount(raceID, p.Player, numToAdd)
    if err != nil {
        return -1, err
    }

    s.mu.Lock()
	defer s.mu.Unlock()

	//Re-check these after locking again
	p, ok = s.state.Players[playerTwitch]
	if !ok || raceID != s.state.RaceID {
		return -1, errors.New("state changed while changing player count")
	}

    p.NumCollected = float64(newNum)

    //Send this update
	ud := Update{
		Type: PlayerCount,
		Player: p,
		Game: nil,
		Timer: nil,
	}
	for _, sub := range s.subscribers {
		if sub.updateTypes[All] || sub.updateTypes[PlayerCount] {
			select {
			case sub.updateCh <- ud:
			default:
			}
		}
	}

    return newNum, nil
}

func (s *store) SetPlayerCount(numToSet int, playerTwitch string) (int, error) {
	//Make sure player is lowercase
	playerTwitch = strings.ToLower(playerTwitch)

	s.mu.RLock()
    if !s.raceIsLoaded() {
        s.mu.RUnlock()
        return -1, noRaceLoadedErr
    }
    p, ok := s.state.Players[playerTwitch]
    raceID := s.state.RaceID
    s.mu.RUnlock()
    if !ok {
        return -1, errors.New("player is not in this race")
    }

	//Only allow this if player is still running in the race
	if p.Status != "running" {
		return -1, errors.New("can't update player count for player who is no longer in the race")
	}

    newNum, err := mmapi.SetPlayerCount(raceID, p.Player, numToSet)
    if err != nil {
        return -1, err
    }

    s.mu.Lock()
	defer s.mu.Unlock()

	//Re-check these after locking again
	p, ok = s.state.Players[playerTwitch]
	if !ok || raceID != s.state.RaceID {
		return -1, errors.New("state changed while changing player count")
	}

    p.NumCollected = float64(newNum)

	//Send update
    ud := Update{
		Type: PlayerCount,
		Player: p,
		Game: nil,
		Timer: nil,
	}
	for _, sub := range s.subscribers {
		if sub.updateTypes[All] || sub.updateTypes[PlayerCount] {
			select {
			case sub.updateCh <- ud:
			default:
			}
		}
	}

    return newNum, nil
}

//Gets actual player name from their Twitch
func (s *store) GetPlayerNameFromTwitch(playerTwitchName string) (string, error) {
	//Make sure player is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return "", noRaceLoadedErr
	}

	if s.state.Players[playerTwitchName] == nil {
		return "", errors.New("player not in this race")
	}

	return s.state.Players[playerTwitchName].Player, nil
}

func (s *store) UpdatePlayerName(playerTwitchName string, newName string) error {
	//Get player name. Use their display name if possible
	//This isn't necessary but it's faster for the API, but that's another can of worms
	//Twitch names are lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)

	currentName := playerTwitchName
	playerInLoadedRace := false

	s.mu.RLock()
	if s.raceIsLoaded() {
		player, ok := s.state.Players[playerTwitchName]
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
		if p, ok := s.state.Players[playerTwitchName]; ok {
			p.Player = newName
		}
		s.mu.Unlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	//Send update if this player is in the store
	if p, ok := s.state.Players[playerTwitchName]; ok {
		ud := Update{
			Type: PlayerName,
			Player: p,
			Game: nil,
			Timer: nil,
		}

		for _, sub := range s.subscribers {
			if sub.updateTypes[All] || sub.updateTypes[PlayerName] {
				select {
				case sub.updateCh <- ud:
				default:
				}
			}
		}
	}

	return nil
}

func (s *store) UpdateGameTime(playerTwitchName string, gameName string, newTime string) error {
	//Make sure player is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)

	s.mu.Lock()
	if !s.raceIsLoaded() {
		s.mu.Unlock()
		return noRaceLoadedErr
	}

	//Get player name. Use their display name if possible
	//This isn't necessary but it's faster for the API, but that's another can of worms
	currentName := playerTwitchName

	if s.raceIsLoaded() {
		player, ok := s.state.Players[playerTwitchName]
		if ok {
			currentName = player.Player
		}
	}

	gameCatName := categoryinfo.GetGameCategoryFromGameName(s.info.Category, gameName)
	raceID := s.state.RaceID
	s.mu.Unlock()

	//Send the update to the backend
	err := mmapi.UpdatePlayerCategoryTime(raceID, currentName, gameCatName, newTime)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	//Check again for stale state
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	//If race ID or game name has changed, return
	if raceID != s.state.RaceID || gameCatName != categoryinfo.GetGameCategoryFromGameName(s.info.Category, gameName) {
		return errors.New("race state changed during update")
	}

	//Send update to subscribers
	ud := Update{
		Type: GameTime,
		Player: nil,
		Timer: nil,
		Game: &GameInfo{
			GameName: gameCatName,
			GameTime: newTime,
		},
	}
	for _, sub := range s.subscribers {
		if sub.updateTypes[All] || sub.updateTypes[GameTime] {
			select {
			case sub.updateCh <- ud:
			default:
			}
		}
	}

	return nil
}

func (s *store) UpdateFinalTime(playerTwitchName string, newTime string) error {
	//Make sure player is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)

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
		player, ok := s.state.Players[playerTwitchName]
		if ok {
			currentName = player.Player
			playerInLoadedRace = true
		}
	}
	raceID := s.state.RaceID
	s.mu.RUnlock()

	//Send the update to the backend
	err := mmapi.UpdatePlayerFinalTime(raceID, currentName, newTime)
	if err != nil {
		return err
	}
	
	//Update player's time and send it to the stats page if they are in the loaded race
	if playerInLoadedRace {
		s.mu.Lock()
		if p, ok := s.state.Players[playerTwitchName]; ok {
			p.FinalTime = newTime
			ud := Update{
				Type: FinalTime,
				Player: p,
				Game: nil,
				Timer: nil,
			}

			for _, sub := range s.subscribers {
				if sub.updateTypes[All] || sub.updateTypes[FinalTime] {
					select{
					case sub.updateCh <- ud:
					default:
					}
				}
			}
		}
		s.mu.Unlock()
	}

	return nil
}

//Sets player's status to custom value
func (s *store) SetPlayerStatus(playerTwitchName string, newStatus string) error {
	//Make sure player is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)

	s.mu.Lock()
	if !s.raceIsLoaded() {
		s.mu.Unlock()
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.state.Players[playerTwitchName]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("%s is not in this race", playerTwitchName)
	}
	p.Status = newStatus
	
	//Send update
	ud := Update{
		Type: PlayerStatus,
		Player: p,
		Timer: nil,
		Game: nil,
	}
	for _, sub := range s.subscribers {
		if sub.updateTypes[All] || sub.updateTypes[PlayerStatus] {
			select {
			case sub.updateCh <- ud:
			default:
			}
		}
	}
	s.mu.Unlock()

	//Cache this status
	go s.cachePlayerStatus(playerTwitchName, cacheDir)
	return nil
}

func (s *store) GetPlayerStatus(playerTwitchName string) (string, error) {
	playerTwitchName = strings.ToLower(playerTwitchName)

	//Get race information
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.raceIsLoaded() {
		return "", noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.state.Players[playerTwitchName]
	if !ok {
		return "", fmt.Errorf("%s is not in this race", playerTwitchName)
	}

	return p.Status, nil
}

func (s *store) PlayerIsFinished(playerTwitchName string) (bool, error) {
	playerTwitchName = strings.ToLower(playerTwitchName)

	//Get race information
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.raceIsLoaded() {
		return false, noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.state.Players[playerTwitchName]
	if !ok {
		return false, fmt.Errorf("%s is not in this race", playerTwitchName)
	}

	return p.NumCollected >= float64(categoryinfo.GetTotalCollectiblesFromCategoryName(s.info.Category)), nil
}

//Sets player's status to custom value
func (s *store) SetTimerValue(newTimerValue string) error {
	s.mu.Lock()
	if !s.raceIsLoaded() {
		s.mu.Unlock()
		return noRaceLoadedErr
	}

	//Get start time from timer value
	newSTime, err := getStartTimeFromTimeString(newTimerValue)
	if err != nil {
		s.mu.Unlock()
		return err //No changes
	}
	s.state.Timer.StartTime = newSTime

	//Send update
	ud := Update{
		Type: StartTime,
		Player: nil,
		Game: nil,
		Timer: s.state.Timer,
	}
	for _, sub := range s.subscribers {
		if sub.updateTypes[All] || sub.updateTypes[StartTime] {
			select {
			case sub.updateCh <- ud:
			default:
			}
		}
	}
	s.mu.Unlock()
	
	//Cache timer status
	go s.cacheTimerInfo(cacheDir)

	return nil
}

//Stops the timer
func (s *store) StopTimer() error {
	s.mu.Lock()
	if !s.raceIsLoaded() {
		s.mu.Unlock()
		return noRaceLoadedErr
	}

	//Update the timer if necessary
	if s.state.Timer.TimerRunning {
		s.state.Timer.TimerRunning = false
		
		//Send the update
		ud := Update{
			Type: TimerState,
			Player: nil,
			Game: nil,
			Timer: s.state.Timer,
		}
		for _, sub := range s.subscribers {
			if sub.updateTypes[All] || sub.updateTypes[TimerState] {
				select {
				case sub.updateCh <- ud:
				default:
				}
			}
		}
	}
	s.mu.Unlock()
	
	//Cache timer status
	go s.cacheTimerInfo(cacheDir)
	return nil
}

//Starts the timer
func (s *store) StartTimer() error {
	s.mu.Lock()
	if !s.raceIsLoaded() {
		s.mu.Unlock()
		return noRaceLoadedErr
	}

	//Update the timer if necessary
	if !s.state.Timer.TimerRunning {
		s.state.Timer.TimerRunning = true
		
		//Send the update
		ud := Update{
			Type: TimerState,
			Player: nil,
			Game: nil,
			Timer: s.state.Timer,
		}
		for _, sub := range s.subscribers {
			if sub.updateTypes[All] || sub.updateTypes[TimerState] {
				select {
				case sub.updateCh <- ud:
				default:
				}
			}
		}
	}

	s.mu.Unlock()
	
	//Cache timer status
	go s.cacheTimerInfo(cacheDir)

	return nil
}

//Adds a user as an organizer
func AddOrganizer(playerTwitchName string) {
	//Make sure twitch name is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)
	organizerMu.Lock()
	organizerList[playerTwitchName] = true
	organizerMu.Unlock()

	go saveOrganizerList(organizersPath)
}

//Removes user from organizer list
func RemoveOrganizer(playerTwitchName string) {
	//Make sure player is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)
	
	organizerMu.Lock()
	organizerList[playerTwitchName] = false
	organizerMu.Unlock()

	go saveOrganizerList(organizersPath)
}

//Checks if a player is an organizer by twitch name
func IsOrganizer(playerTwitchName string) bool {
	//Make sure player is lowercase
	playerTwitchName = strings.ToLower(playerTwitchName)

	organizerMu.RLock()
	defer organizerMu.RUnlock()

	return organizerList[playerTwitchName]
}

//Adds a user to blacklist
func AddBlacklistUser(user string) {
	//Make sure player is lowercase
	user = strings.ToLower(user)

	blacklistMu.Lock()
	blacklist[user] = true

	//This will also remove this person from the whitelist
	whitelistUpdate := false
	whitelistMu.Lock()
	if whitelist[user] {
		whitelist[user] = false
		whitelistUpdate = true
	}
	whitelistMu.Unlock()

	blacklistMu.Unlock()

	go saveBlacklist(blacklistPath)
	if whitelistUpdate {
		go saveWhitelist(whitelistPath)
	}
}

//Removes a user from blacklist
func RemoveBlacklistUser(user string) {
	//Make sure player is lowercase
	user = strings.ToLower(user)

	blacklistMu.Lock()
	blacklist[user] = false
	blacklistMu.Unlock()

	go saveBlacklist(blacklistPath)
}

//Checks if a player is on blacklist by twitch name
func IsOnBlacklist(user string) bool {
	//Make sure player is lowercase
	user = strings.ToLower(user)

	blacklistMu.RLock()
	defer blacklistMu.RUnlock()

	return blacklist[user]
}

//Adds a user to whitelist
func AddWhitelistUser(user string) {
	//Make sure player is lowercase
	user = strings.ToLower(user)

	whitelistMu.Lock()
	whitelist[user] = true
	whitelistMu.Unlock()

	go saveWhitelist(whitelistPath)
}

//Removes a user from whitelist
func RemoveWhitelistUser(user string) {
	//Make sure player is lowercase
	user = strings.ToLower(user)

	whitelistMu.Lock()
	whitelist[user] = false
	whitelistMu.Unlock()

	go saveWhitelist(whitelistPath)
}

//Checks if a player is on whitelist by twitch name
func IsOnWhitelist(user string) bool {
	//Make sure player is lowercase
	user = strings.ToLower(user)

	whitelistMu.RLock()
	defer whitelistMu.RUnlock()

	return whitelist[user]
}

//Helper to save organizer list
func saveOrganizerList(filePath string) {
	organizerMu.Lock()
	defer organizerMu.Unlock()
	organizerFile, err := os.OpenFile(filePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Error updating organizer list:%s\n", err.Error()) //Couldn't write, it's no biggie
		return
	}
	defer organizerFile.Close()

	//Write our data
	for n, b := range organizerList {
		if b {
			_, err = fmt.Fprintln(organizerFile, n)
			if err != nil {
				fmt.Printf("Error updating organizer list:%s\n", err.Error()) //Couldn't write, it's no biggie
				continue
			}
		}
	}
}

//Saves blackist
func saveBlacklist(filePath string) {
	blacklistMu.Lock()
	defer blacklistMu.Unlock()
	blacklistFile, err := os.OpenFile(filePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Error updating blacklist: %s\n", err.Error()) //Couldn't write, it's no biggie
		return
	}
	defer blacklistFile.Close()

	//Write our data
	for n, b := range blacklist {
		if b {
			_, err = fmt.Fprintln(blacklistFile, n)
			if err != nil {
				fmt.Printf("Error updating blacklist: %s\n", err.Error()) //Couldn't write, it's no biggie
				continue
			}
		}
	}
}

//Saves whitelist
func saveWhitelist(filePath string) {
	whitelistMu.Lock()
	defer whitelistMu.Unlock()
	whitelistFile, err := os.OpenFile(filePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Error updating whitelist: %s\n", err.Error()) //Couldn't write, it's no biggie
		return
	}
	defer whitelistFile.Close()

	//Write our data
	for n, b := range whitelist {
		if b {
			_, err = fmt.Fprintln(whitelistFile, n)
			if err != nil {
				fmt.Printf("Error updating whitelist: %s\n", err.Error()) //Couldn't write, it's no biggie
				continue
			}
		}
	}
}

//Getter for getting current race category
func (s *store) GetCurrentRaceCategory() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.raceIsLoaded() {
		return "", noRaceLoadedErr
	}
	return s.info.Category, nil
}

//Gets mmapi raceinfo for control panel
func (s *store) GetStoredRaceInfo() (*mmapi.RaceInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

func (s *store) GetCurrentRaceID() (int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.RaceID
}

//Updates status of current race
func (s *store) UpdateRaceStatus(newStatus string) error {
	//Start the race
	s.mu.RLock()
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}
	raceID := s.state.RaceID
	s.mu.RUnlock()

	err := mmapi.UpdateRaceStatus(raceID, newStatus)
	if err != nil {
		return err
	}

	//Update race cache
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	if raceID != s.state.RaceID {
		return errors.New("race state changed during update")
	}
	s.info.Status = newStatus

	return nil
}

func (s *store) ExportTimes() error {
	s.mu.RLock()
	if !s.raceIsLoaded() {
		s.mu.RUnlock()
		return noRaceLoadedErr
	}
	raceID := s.state.RaceID
	raceStatus := s.info.Status
	s.mu.RUnlock()

	//If this race is in progress, return error
	if raceStatus == "in_progress" {
		return errors.New("can't export times for a race that is in progress")
	}

	//Get player records from backend
	records, err := mmapi.GetPlayersForRace(raceID)
	if err != nil {
		return err
	}

	//Format output
	type gameTime struct {
		CatName string `json:"category"`
		Time string `json:"time"`
	}
	type racer struct {
		Name string `json:"player"`
		Collectibles int `json:"collectibles"`
		FinalTime string `json:"final_time"`
		Games []*gameTime `json:"games"`
	}

	data := make([]*racer, 0)
	dataMap := make(map[string]*racer)

	//Parse records
	twitchNameMap := make(map[string]string)
	names := make([]string, 0)
	for _, r := range records {
		names = append(names, r.Player)
		twitchNameMap[r.Player] = r.PlayerTwitch

		newRacer := &racer{
			Name: r.PlayerTwitch,
			Collectibles: int(r.NumCollected),
			FinalTime: r.FinalTime,
			Games: nil,
		}

		data = append(data, newRacer)
		dataMap[r.PlayerTwitch] = newRacer
	}

	//Get runs for these names
	runs, err := mmapi.GetRunsForPlayers(raceID, names)
	if err != nil {
		return err
	}

	//Loop through runs and finish the data
	for _, r := range runs {
		//Get this player's data
		playerTwitch := twitchNameMap[r.Player]
		playerInfo, exists := dataMap[playerTwitch]
		if !exists {
			continue
		}

		//Add this game time
		if playerInfo.Games == nil {
			playerInfo.Games = make([]*gameTime, 0)
		}
		playerInfo.Games = append(playerInfo.Games, &gameTime{r.GameCategory, r.Time})
	}

	//Marshall data
	dataBytes, err := json.MarshalIndent(data, "", "	")
	if err != nil {
		return err
	}

	//Open file and export
	exportedTimesMu.Lock()
	defer exportedTimesMu.Unlock()
	cacheFile, err := os.OpenFile(exportedTimesPath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	_, err = cacheFile.Write(dataBytes)	

	return err
}

//Takes a player name and gets their placement. Returns a string of place with the suffix (1st, 2nd, etc.)
func (s *store) GetPlayerPlacement(playerTwitch string) (string, error) {
	//Lowercase Twitch name
	playerTwitch = strings.ToLower(playerTwitch)
	
	//Read data from race state
	s.mu.RLock()
	//Make sure player is still in the race
	if _, ok := s.state.Players[strings.ToLower(playerTwitch)]; !ok {
		s.mu.RUnlock()
		return "", fmt.Errorf("%s is not in this race.", playerTwitch)
	}

	//Copy the player map for sorting
	playersSlice := slices.Collect(maps.Values(s.state.Players))
	catName := s.info.Category
	raceID := s.state.RaceID
	s.mu.RUnlock()

	//TODO: maybe change this? This is only like 2n + nlogn each time the function is called, so its' not awful,
	//but if its a bottleneck a separate data structure like a heap or something for rankings could be nice
	//Sort players based on their placement
	slices.SortFunc(playersSlice, func(a, b *PlayerInfo) int {
		return playerRankingSortingFunc(a, b, catName)
	})

	//Get the player's ranking using the sorted slice
	placementMap := getPlayerPlacementMap(playersSlice, catName)

	//Finally, make sure race is the same and player is still in the race
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.state.Players[strings.ToLower(playerTwitch)]; !ok {
		return "", fmt.Errorf("%s is not in this race.", playerTwitch)
	}

	if catName != s.info.Category || raceID != s.state.RaceID {
		return "", errors.New("Can't get player placement, race state has changed")
	}

	//Convert the placement into a string with the suffix
	out := fmt.Sprintf("%v%s", placementMap[playerTwitch], getPlacementSuffix(placementMap[playerTwitch]))
	return out, nil
}

//Gets the players at the given placement
func (s *store) GetPlayersAtPlacement(placement int) ([]string, error) {
	//Copy the player map for sorting
	s.mu.RLock()
	playersSlice := slices.Collect(maps.Values(s.state.Players))
	catName := s.info.Category
	raceID := s.state.RaceID
	s.mu.RUnlock()

	//Sort the players slice
	slices.SortFunc(playersSlice, func(a, b *PlayerInfo) int {
		return playerRankingSortingFunc(a, b, catName)
	})
	
	//Get placement map
	placementMap := getPlayerPlacementMap(playersSlice, catName)

	//Finally, make sure race is the same and player is still in the race
	s.mu.RLock()
	defer s.mu.RUnlock()

	if catName != s.info.Category || raceID != s.state.RaceID {
		return nil, errors.New("Can't get player placement, race state has changed")
	}

	//Get players with this rank. O(n) but nobody uses this command and its 3am idc
	out := make([]string, 0)
	for name, rank := range placementMap {
		if rank == placement {
			out = append(out, name)
		}
	}

	return out, nil
}

//Returns a copy of the current organizer list
func (s *store) GetOrganizerList() map[string]bool {
	organizerMu.RLock()
	defer organizerMu.RUnlock()

	out := maps.Clone(organizerList)

	return out
}

//Returns a copy of the current blacklist
func (s *store) GetBlacklist() map[string]bool {
	blacklistMu.RLock()
	defer blacklistMu.RUnlock()

	out := maps.Clone(blacklist)
	return out
}

//Syncs player data from the backend
func (s *store) syncToMMAPI() error {
	return nil
}

//Gets cached state of this race
func getCachedState(raceID int, cacheDir string) cachedState {
	//Attempt to open cache
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachePath := fmt.Sprintf("%s/%v.json", cacheDir, raceID)
	cacheFile, err := os.Open(cachePath)
	if err != nil {
		return cachedState{Players: make(map[string]*cachedPlayerInfo)} //Return state with empty players
	}

	//Otherwise, decode the cached state and return it
	var currentState cachedState
	err = json.NewDecoder(cacheFile).Decode(&currentState)
	if err != nil {
		return cachedState{Players: make(map[string]*cachedPlayerInfo)} //Return state with empty players
	}

	return currentState
}

//Gets s lock, so make sure it's unlocked before calling
func (s *store) cachePlayerStatus(playerTwitch string, cacheDir string) error {
	//Make sure player is lowercase
	playerTwitch = strings.ToLower(playerTwitch)

	s.mu.RLock()
	raceID := s.state.RaceID
	player, ok := s.state.Players[playerTwitch]
	if !ok {
		s.mu.RUnlock()
		return errors.New("can't save player status: player not in currently loaded state")
	}
	newStatus := player.Status
	s.mu.RUnlock()

	cachePath := fmt.Sprintf("%s/%v.json", cacheDir, raceID)
	
	cacheMu.Lock()
	defer cacheMu.Unlock()
	
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating cache directory: %v", err)
	}

	//Read currently cached file
	var currentState cachedState
	existingData, err := os.ReadFile(cachePath)
	if err != nil {
		//Check if this is becasue the file doesn't exist or there's a different error
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		//No errors, file exists, read it
		json.Unmarshal(existingData, &currentState)
	}

	//If we didn't read any data, create an empty cached state
	if currentState.Players == nil {
		currentState.Players = make(map[string]*cachedPlayerInfo)
	}

	//Update this player's status 
	savedPlayer, exists := currentState.Players[playerTwitch]
	if !exists {
		savedPlayer = &cachedPlayerInfo{}
		currentState.Players[playerTwitch] = savedPlayer
	}
	savedPlayer.Status = newStatus

	//Open file and write back state
	cacheFile, err := os.OpenFile(cachePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	marshalledData, err := json.MarshalIndent(currentState, "", " ")
	if err != nil {
		return err
	}

	_, err = cacheFile.Write(marshalledData)
	if err != nil {
		return err
	}

	return nil
}

func (s *store) cacheTimerInfo(cacheDir string) error {
	s.mu.RLock()
	raceID := s.state.RaceID
	timer := s.state.Timer
	if timer == nil {
		s.mu.RUnlock()
		return errors.New("can't save timer status: timer not initialized")
	}
	newTimerStart := timer.StartTime
	newTimerState := timer.TimerRunning
	s.mu.RUnlock()

	cachePath := fmt.Sprintf("%s/%v.json", cacheDir, raceID)
	
	cacheMu.Lock()
	defer cacheMu.Unlock()
	
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating cache directory: %v", err)
	}

	//Read currently cached file
	var currentState cachedState
	existingData, err := os.ReadFile(cachePath)
	if err != nil {
		//Check if this is becasue the file doesn't exist or there's a different error
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		//No errors, file exists, read it
		json.Unmarshal(existingData, &currentState)
	}

	//If we didn't read any data, create an empty cached state
	if currentState.Timer == nil {
		currentState.Timer = &cachedTimerInfo{}
	}

	//Update timer's state
	currentState.Timer.StartTime = newTimerStart
	currentState.Timer.TimerRunning = newTimerState

	//Open file and write back state
	cacheFile, err := os.OpenFile(cachePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	marshalledData, err := json.MarshalIndent(currentState, "", " ")
	if err != nil {
		return err
	}

	_, err = cacheFile.Write(marshalledData)
	if err != nil {
		return err
	}

	return nil
}

//Helper to check that a race is loaded
func (s *store) raceIsLoaded() bool {
	if s.state.RaceID == -1 || s.state.Players == nil {
		return false
	}

	return true
}

//Loads blacklist
func (s *store) loadBlacklist(filePath string) {
	blacklistMu.Lock()
	defer blacklistMu.Unlock()
	blacklistFile, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error loading blacklist: %s\n", err.Error())
		return
	}
	defer blacklistFile.Close()

	scanner := bufio.NewScanner(blacklistFile)
	for scanner.Scan() {
		blacklist[strings.ToLower(scanner.Text())] = true
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading blacklist: %s\n", err.Error())
	}
}

//Loads organizer list
func (s *store) loadOrganizerList(filePath string) {
	organizerMu.Lock()
	defer organizerMu.Unlock()
	organizerListFile, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error loading organizer list: %s\n", err.Error())
		return
	}
	defer organizerListFile.Close()

	scanner := bufio.NewScanner(organizerListFile)
	for scanner.Scan() {
		organizerList[strings.ToLower(scanner.Text())] = true
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading organizer List: %s\n", err.Error())
	}
}


//Loads whitelist
func (s *store) loadWhitelist(filePath string) {
	//Racers and organizers are already on the whitelist
	tempWhitelist := make(map[string]bool)

	//Add players and organizers to whitelist
	s.mu.RLock()
	blacklistMu.RLock()
	for _, player := range s.state.Players {
		tempWhitelist[player.PlayerTwitch] = !blacklist[player.PlayerTwitch]
	}
	blacklistMu.RUnlock()
	s.mu.RUnlock()

	organizerMu.RLock()
	for o, b := range organizerList {
		if b {
			tempWhitelist[o] = true
		}
	}
	organizerMu.RUnlock()

	whiteListFile, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error loading whitelist: %s\n", err.Error())
	} else {
		scanner := bufio.NewScanner(whiteListFile)
		for scanner.Scan() {
			tempWhitelist[strings.ToLower(scanner.Text())] = true
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading whiteList: %s\n", err.Error())
		}
		whiteListFile.Close()
	}

	//Copy our temp whitelist to the actual whitelist
	whitelistMu.Lock()
	maps.Copy(whitelist, tempWhitelist)
	whitelistMu.Unlock()

	//Save organizers and racers to whitelist. Racers just get the free whitelist pass
	go saveWhitelist(whitelistPath)
}

//Helper function for logging useful racer state information
func (s *store) LogPlayerState(playerName string, playerNum int) {
	//Get information from store
	s.mu.RLock()
	currentStatus := s.info.Status
	date := s.info.Date
	s.mu.RUnlock()

	//Only do this log if the race is in progress
	if currentStatus != "in_progress" {
		return	
	}

	layout := "2006-01-02T15:04:05"
	currentTime := time.Now().Format(layout)
	
	logStatesMu.Lock()
	defer logStatesMu.Unlock()

	err := os.MkdirAll(logStatesDir, 0755)
	if err != nil {
		fmt.Printf("error creating state log directory: %v", err)
		return
	}

	filePath := fmt.Sprintf("%s%s-state-username.log", logStatesDir, date)

	//Open file for appending
	stateFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %s\n", err.Error())
		return
	}
	defer stateFile.Close()

	b := []byte(fmt.Sprintf("%s %s %v\n", currentTime, playerName, playerNum))
	stateFile.Write(b)
}

//Gets start time in unix millis from timer string
func getStartTimeFromTimeString(timeString string) (int64, error) {
	//Parse timer values
	parts := strings.Split(timeString, ":")
	if len(parts) != 3 {
		return -1, errors.New("not a valid time string")
	}

	hoursNum, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1, err
	}

	minutesNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1, err
	}

	secondsNum, err := strconv.Atoi(parts[2])
	if err != nil {
		return -1, err
	}

	totalMillis := int64(hoursNum) * 3600 * 1000 + int64(minutesNum) * 60 * 1000 + int64(secondsNum) * 1000
	startTime := time.Now().UnixMilli() - totalMillis //Get start time in millis
	return startTime, nil
}

//Helper to sort players based on their race progress
func playerRankingSortingFunc(a *PlayerInfo, b *PlayerInfo, category string) int {
	totalCatCollectibles := categoryinfo.GetTotalCollectiblesFromCategoryName(category)

	aFinished := a.NumCollected >= float64(totalCatCollectibles)
	bFinished := b.NumCollected >= float64(totalCatCollectibles)

	//Both players finish, lowest final time wins. Otherwise, go by alphabetical order
	if aFinished && bFinished {
		res := cmp.Compare(a.FinalTime, b.FinalTime)
		if res == 0 {
			res = cmp.Compare(a.Estimate, b.Estimate)
		}
		if res == 0 {
			res = cmp.Compare(a.PlayerTwitch, b.PlayerTwitch)
		}

		return res
	}

	//Finishers rank before non-finishers
	if aFinished { return -1 }
	if bFinished { return 1 }

	aQuit := a.Status != "running"
	bQuit := b.Status != "running"

	//Sort by num collected first, in a tie the quitter goes second
	if aQuit && !bQuit {
		res := cmp.Compare(b.NumCollected, a.NumCollected)
		if res == 0 {
			res = 1
		}
		return res
	}

	if bQuit && !aQuit {
		res := cmp.Compare(b.NumCollected, a.NumCollected)
		if res == 0 {
			res = -1
		}
		return res
	}

	//Finally, sort normally if both players are still going
	res := cmp.Compare(b.NumCollected, a.NumCollected)
	if res == 0 {
		res = cmp.Compare(a.FinalTime, b.FinalTime)
	}
	if res == 0 {
		res = cmp.Compare(a.Estimate, b.Estimate)
	}
	if res == 0 {
		res = cmp.Compare(a.PlayerTwitch, b.PlayerTwitch)
	}

	return res
}

//Helper to return the map of players and their placement
func getPlayerPlacementMap(sortedPlayerList []*PlayerInfo, category string) map[string]int {
	totalCatCollectibles := categoryinfo.GetTotalCollectiblesFromCategoryName(category)
	res := make(map[string]int)

	for i, p := range sortedPlayerList {
		//First index is 1st place no matter what
		if i == 0 {
			res[p.PlayerTwitch] = i + 1
			continue
		}

		prevPlayer := sortedPlayerList[i - 1]

		prevFinished := prevPlayer.NumCollected >= float64(totalCatCollectibles)
		pFinished := p.NumCollected >= float64(totalCatCollectibles)

		//Both players are finished, tie them if times are the same
		if prevFinished && pFinished {
			if prevPlayer.FinalTime == p.FinalTime {
				res[p.PlayerTwitch] = res[prevPlayer.PlayerTwitch]
			} else {
				res[p.PlayerTwitch] = i + 1
			}
			continue
		}

		//If one player has quit, the quitter is always behind
		prevQuit := prevPlayer.Status != "running"
		pQuit := p.Status != "running"
		if !prevQuit && pQuit {
			res[p.PlayerTwitch] = i + 1
			continue
		}

		if prevPlayer.NumCollected == p.NumCollected {
			res[p.PlayerTwitch] = res[prevPlayer.PlayerTwitch]
			continue
		}

		//Finally, go by index
		res[p.PlayerTwitch] = i + 1
	}

	return res
}

//Helper to get suffix for a number
func getPlacementSuffix(num int) string {
	if num % 100 >= 11 && num % 100 <= 13 {
		return "th"
	}

	switch num % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}