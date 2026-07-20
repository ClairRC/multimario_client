package store

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
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

var exportedTimesMu sync.RWMutex

var cacheDir string = "./cache"
var organizersPath = "./organizers.txt"
var blacklistPath = "./blacklist.txt"
var exportedTimesPath = "./results.json"

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

	//Load organizer and blacklist
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
	defer s.mu.Unlock()
	s.state = newState
	s.info = raceInfo

	//Send new state to subscribers
	for _, sub := range s.subscribers {
		select {
			case sub.stateCh <- *newState:
			default:
		}
	}

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
	organizerMu.Lock()
	organizerList[playerTwitchName] = true
	organizerMu.Unlock()

	go saveOrganizerList(organizersPath)
}

//Removes user from organizer list
func RemoveOrganizer(playerTwitchName string) {
	organizerMu.Lock()
	organizerList[playerTwitchName] = false
	organizerMu.Unlock()

	go saveOrganizerList(organizersPath)
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
	blacklistMu.Unlock()

	go saveBlacklist(blacklistPath)
}

//Removes a user from blacklist
func RemoveBlacklistUser(user string) {
	blacklistMu.Lock()
	blacklist[user] = false
	blacklistMu.Unlock()

	go saveBlacklist(blacklistPath)
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

//Checks if a player is an organizer by twitch name
func IsOnBlacklist(user string) bool {
	blacklistMu.RLock()
	defer blacklistMu.RUnlock()

	return blacklist[user]
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
	type timeData struct {
		FinalTime string `json:"final_time"`
		Games []*gameTime `json:"games"`
	}
	data := make(map[string]*timeData)

	//Parse records
	twitchNameMap := make(map[string]string)
	names := make([]string, 0)
	for _, r := range records {
		names = append(names, r.Player)
		twitchNameMap[r.Player] = r.PlayerTwitch
		data[r.PlayerTwitch] = &timeData{
			FinalTime: r.FinalTime,
			Games: nil,
		}
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
		playerInfo, exists := data[playerTwitch]
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
		blacklist[scanner.Text()] = true
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
		organizerList[scanner.Text()] = true
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading organizer List: %s\n", err.Error())
	}
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