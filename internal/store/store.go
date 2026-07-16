package store

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

var Race = &store{
	state: &State{RaceID: -1, Players: nil, Timer: &TimerInfo{false, time.Now().UnixMilli()}, Category: ""},
	subscribers: make(map[int]*subscriber),
	info: nil,
	next: 0,
}

var noRaceLoadedErr error = errors.New("no race is currently loaded")

var organizerMu sync.RWMutex
var organizerList map[string]bool = map[string]bool{"clairdss": true}

var blacklistMu sync.RWMutex
var blacklist map[string]bool = make(map[string]bool)

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
	//TODO: Add functionality for loading cached state from json perchance
	newState := &State{
		RaceID: raceID,
		Players: playerNameMap,
		Timer: &TimerInfo{false, time.Now().UnixMilli()},
		Category: raceInfo.Category,
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

//Sets player's status to quit
func (s *store) SetToQuit(playerTwitchName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.state.Players[playerTwitchName]
	if !ok {
		return fmt.Errorf("%s is not in this race", playerTwitchName)
	}
	p.Status = "Forfeit"
	
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

	return nil
}

//Sets player's status to not quit (running)
func (s *store) SetToRunning(playerTwitchName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.state.Players[playerTwitchName]
	if !ok {
		return fmt.Errorf("%s is not in this race", playerTwitchName)
	}
	p.Status = "running"
	
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

	return nil
}

//Sets player's status to custom value
func (s *store) SetPlayerStatus(playerTwitchName string, newStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	//Only do this if the racer is in the race
	p, ok := s.state.Players[playerTwitchName]
	if !ok {
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

	return nil
}

//Sets player's status to custom value
func (s *store) SetTimerValue(newTimerValue string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.raceIsLoaded() {
		return noRaceLoadedErr
	}

	//Get start time from timer value
	newSTime, err := getStartTimeFromTimeString(newTimerValue)
	if err != nil {
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

	return nil
}

//Stops the timer
func (s *store) StopTimer() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.raceIsLoaded() {
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

	return nil
}

//Starts the timer
func (s *store) StartTimer() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.raceIsLoaded() {
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
	blacklistMu.Unlock()
}

//Removes a user from blacklist
func RemoveBlacklistUser(user string) {
	blacklistMu.Lock()
	blacklist[user] = false
	blacklistMu.Unlock()
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
	raceID := s.state.RaceID
	s.mu.RUnlock()

	err := mmapi.UpdateRaceStatus(s.state.RaceID, newStatus)
	if err != nil {
		return err
	}

	//Update race cache
	s.mu.Lock()
	defer s.mu.Unlock()
	if raceID != s.state.RaceID {
		return errors.New("race state changed during update")
	}
	s.info.Status = newStatus

	return nil
}


//Helper to check that a race is loaded
func (s *store) raceIsLoaded() bool {
	if s.state.RaceID == -1 || s.state.Players == nil {
		return false
	}

	return true
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