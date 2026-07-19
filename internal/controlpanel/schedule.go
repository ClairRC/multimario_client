package controlpanel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch/chat"
)

//File to handle scheduling races

type raceSchedule struct {
	ctx context.Context
	cancel context.CancelFunc
	mu sync.RWMutex
	raceID int
	startTime time.Time
	raceLimit time.Duration
	updateStartRaceTimeCh chan(time.Time)
	updateStartStreamTimeCh chan(time.Time)
}

const NoTimeLimit time.Duration = 0
const streamStartOffset = time.Duration(1 * time.Minute)
const streamEndOffset = time.Duration(1 * time.Minute)

var Schedule = raceSchedule{}
var cacheMu sync.RWMutex
var scheduleCachePath = "./schedule.json"

//Takes start time and raceID and schedules the race. Takes start time as rfc3339
func (s *raceSchedule) ScheduleRace(raceID int, startTime string, timeLimit string) error {
	//Give Schedule channels to update different parameterse

	//Validate fields
	startTimeUTC := startTime + "Z" //UTC time
	startTimeStruct, err := time.Parse(time.RFC3339, startTimeUTC)
	if err != nil {
		return errors.New("unable to parse start time in RFC3339 format")
	}

	//Check time limit
	timeLimitStruct, err := parseTimeDuration(timeLimit)
	if timeLimit != "" && err != nil {
		return errors.New("race time limit is invalid time string")
	}

	//Only schedule this race if the total race time has not passed
	if startTimeStruct.Add(timeLimitStruct).Add(streamEndOffset).Before(time.Now()) {
		return errors.New("race time has passed. no race will be scheduled")
	}

	s.mu.Lock()
	//Schedule can only be set for 1 race at a time
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	s.raceID = raceID
	s.updateStartRaceTimeCh = make(chan(time.Time))
	s.updateStartStreamTimeCh = make(chan(time.Time))
	s.raceLimit = timeLimitStruct
	s.startTime = startTimeStruct

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	go s.waitForStreamStart(ctx, cancel)
	go s.waitForRaceStart(ctx)
	s.mu.Unlock()

	err = s.storeSchedule(scheduleCachePath)
	if err != nil {
		logMessage(fmt.Sprintf("Error storing scheduled race: %s Schedule was not saved.", err.Error()))
	}

	s.logRace()

	return nil
}

//Unschedules race and deletes the current race
func (s *raceSchedule) UnscheduleRace() error {
	s.mu.Lock() 
	removedRaceID := s.raceID 
	if s.cancel == nil {
		s.mu.Unlock()
		return errors.New("cannot unschedule race: no race scheduled")
	}

	s.cancel()

	s.raceID = -1
	s.ctx = nil
	s.cancel = nil
	s.updateStartRaceTimeCh = nil
	s.updateStartStreamTimeCh = nil
	s.mu.Unlock()

	go s.deleteSchedule(removedRaceID, scheduleCachePath)

	return nil
}

func (s *raceSchedule) UpdateStartTime(newStart string) error {
	startTimeUTC := newStart + "Z" //UTC time
	startTimeStruct, err := time.Parse(time.RFC3339, startTimeUTC)
	if err != nil {
		return errors.New("unable to parse start time in RFC3339 format")
	}

	s.mu.Lock()
	if s.ctx == nil {
		s.mu.Unlock()
		return errors.New("no race is currently scheduled")
	}

	ctx := s.ctx
	updateRaceCh := s.updateStartRaceTimeCh
	updateStreamCh := s.updateStartStreamTimeCh
	s.startTime = startTimeStruct
	s.mu.Unlock()

	select {
	case updateRaceCh <- startTimeStruct:
		logMessage("Scheduled race start time has been updated")
	case <- ctx.Done():
		return errors.New("scheduled race has been cancelled")
	default:
	}

	select {
	case updateStreamCh <- startTimeStruct:
		logMessage("Scheduled stream start time has been updated")
	case <- ctx.Done():
		return errors.New("scheduled race has been cancelled")
	default:
	}

	return nil
}

func (s *raceSchedule) UpdateRaceTimeLimit(newLimit string) error {
	//Check time limit
	timeLimitStruct, err := parseTimeDuration(newLimit)
	if newLimit != "" && err != nil {
		return errors.New("race time limit is invalid time string")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		return errors.New("no race is currently scheduled")
	}

	s.raceLimit = timeLimitStruct	
	return nil
}

//Starts stream and connects to Twitch 30 minutes before the start time
func (s *raceSchedule) waitForStreamStart(ctx context.Context, cancel context.CancelFunc) {
	//Set channels for updates
	s.mu.RLock()
	updateStartTimeCh := s.updateStartStreamTimeCh

	timer := time.NewTimer(time.Until(s.startTime.Add(-1 * streamStartOffset)))
	defer timer.Stop()
	s.mu.RUnlock()

	for {
		select {
		case <-timer.C:
			//Timer fires, start race
			err := s.connectToTwitchForScheduledRace(ctx)
			if err != nil {
				logMessage(fmt.Sprintf("Error connecting to Twitch for scheduled race. Race will be cancelled: %s", err.Error()))
				cancel()
			}
			return

		case newStartTime := <-updateStartTimeCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(time.Until(newStartTime.Add(-1 * streamStartOffset)))

		case <-ctx.Done():
			return
		}
	}
}

//Starts the race at the provided start time
func (s *raceSchedule) waitForRaceStart(ctx context.Context) {
	//Set channels for updates
	s.mu.RLock()
	updateStartTimeCh := s.updateStartRaceTimeCh

	timer := time.NewTimer(time.Until(s.startTime))
	defer timer.Stop()
	s.mu.RUnlock()

	for {
		select {
		case <-timer.C:
			//Timer fires, start race
			err := s.beginScheduledRace(ctx)
			if err != nil {
				logMessage(fmt.Sprintf("Error starting scheduled race: %s", err.Error()))
				return
			}

			//Check if this race is scheduled to finish automatically
			s.mu.RLock()
			raceLimit := s.raceLimit
			s.mu.RUnlock()
			if raceLimit != NoTimeLimit {
				go s.waitForRaceEnd(ctx)
				go s.waitForStreamEnd(ctx)
			}
			return

		case newStartTime := <-updateStartTimeCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(time.Until(newStartTime))

		case <-ctx.Done():
			return
		}
	}
}

func (s *raceSchedule) waitForStreamEnd(ctx context.Context) {
	s.mu.RLock()
	limit := s.raceLimit

	stateCh, updateCh, unsub := store.Race.Subscribe(ctx, store.StartTime)
	defer unsub()

	//Get actual start time
	startTime := s.startTime
	state := <-stateCh
	if state.Timer != nil {
		startTime = time.UnixMilli(state.Timer.StartTime)
	}

	endTime, err := getEndTime(startTime, limit)
	if err != nil {
		s.mu.RUnlock()
		logMessage(fmt.Sprintf("Error scheduling stream end. Bot will not disconnect from Twitch automatically: %s", err.Error()))
		return
	}

	timer := time.NewTimer(time.Until(endTime.Add(streamEndOffset)))
	defer timer.Stop()
	s.mu.RUnlock()

	for {
		select {
		case <-timer.C:
			err := s.endStreamForScheduledRace(ctx)
			if err != nil {
				logMessage(fmt.Sprintf("Unable to disconnect from Twitch: %s", err.Error()))
				return
			}
			logMessage("Bot has disconnected from Twitch.")
			return

		case update := <-updateCh:
			/* Recalculate end time and reset timer */
			effectiveStart := time.UnixMilli(update.Timer.StartTime)
			newEndTime, err := getEndTime(effectiveStart, limit)

			//Only update timer if there's no error
			if err != nil {
				logMessage(fmt.Sprintf("Error disconnecting: %s. Stream will end at the original end time.", err.Error()))
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(time.Until(newEndTime.Add(streamEndOffset)))
			}

		case <-ctx.Done():
			return
		}
	}
}

func (s *raceSchedule) waitForRaceEnd(ctx context.Context) {
	s.mu.RLock()
	limit := s.raceLimit

	stateCh, updateCh, unsub := store.Race.Subscribe(ctx, store.StartTime)
	defer unsub()

	//Get actual start time
	startTime := s.startTime
	state := <-stateCh
	if state.Timer != nil {
		startTime = time.UnixMilli(state.Timer.StartTime)
	}

	endTime, err := getEndTime(startTime, limit)
	if err != nil {
		s.mu.RUnlock()
		logMessage(fmt.Sprintf("Error scheduling race end. Race will not end automatically: %s", err.Error()))
		return
	}

	timer := time.NewTimer(time.Until(endTime))
	defer timer.Stop()
	s.mu.RUnlock()

	for {
		select {
		case <-timer.C:
			err := s.endScheduledRace(ctx)
			if err != nil {
				logMessage(fmt.Sprintf("Error ending race: %s. Race will not end automatically.", err.Error()))
			}
			return

		case update := <-updateCh:
			/* Recalculate end time and reset timer */
			effectiveStart := time.UnixMilli(update.Timer.StartTime)
			newEndTime, err := getEndTime(effectiveStart, limit)

			//Only update timer if there's no error
			if err != nil {
				logMessage(fmt.Sprintf("Error disconnecting: %s. Stream will end at the original end time.", err.Error()))
				continue
			}

			if !newEndTime.After(time.Now()) {
				logMessage("Warning: race ended prematurely due to a timer update")

				if logs, err := chat.GetLog(); err == nil {
					startingIndex := max(len(logs)-20, 0)
					for i := startingIndex; i < len(logs); i++ {
						v := strings.Split(logs[i], " ")
						if len(v) > 1 {
							if cmd := v[1]; cmd == "!settimer" {
								logMessage(logs[i])
							}
						}
					}
				}

				if err := s.endScheduledRace(ctx); err != nil {
					logMessage(fmt.Sprintf("Error ending race: %s. Race will not end automatically.", err.Error()))
				}
				return
			}

			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(time.Until(newEndTime))

		case <-ctx.Done():
			return
		}
	}
}

func (s *raceSchedule) connectToTwitchForScheduledRace(ctx context.Context) error {
	select {
	case <- ctx.Done():
		return errors.New("unable to connect to twitch: Race was cancelled")
	default:
	}

	s.mu.RLock()
	raceID := s.raceID
	s.mu.RUnlock()

	err := selectRace(raceID)
	if err != nil {
		return err
	}

	err = connectToTwitchChat()
	if err != nil {
		return err
	}

	return nil
}

func (s *raceSchedule) beginScheduledRace(ctx context.Context) error {
	select {
	case <- ctx.Done():
		return errors.New("unable to begin race: Race was cancelled")
	default:
	}

	s.mu.RLock()
	raceID := s.raceID
	s.mu.RUnlock()

	err := selectRace(raceID)
	if err != nil {
		return err
	}

	err = startRace()
	if err != nil {
		return err
	}

	go s.deleteSchedule(raceID, scheduleCachePath) //Delete file since race is starting

	return nil
}

func (s *raceSchedule) endStreamForScheduledRace(ctx context.Context) error {
	select{
	case <-ctx.Done():
		return errors.New("unable to disconnect from twitch: race was cancelled")
	default:
	}

	//Disconnect from Twitch
	disconnectFromTwitchChat()

	return nil
}

func (s *raceSchedule) endScheduledRace(ctx context.Context) error {
	select {
	case <- ctx.Done():
		return errors.New("unable to end race: Race was cancelled")
	default:
	}

	s.mu.RLock()
	raceID := s.raceID
	s.mu.RUnlock()

	currentlyLoadedRaceID := store.Race.GetCurrentRaceID()
	if currentlyLoadedRaceID != raceID {
		return errors.New("unable to end race, current race is different from scheduled race")
	}
	
	err := finishRace()
	if err != nil {
		return err
	}

	return nil
}

func getEndTime(startTime time.Time, timeLimit time.Duration) (time.Time, error) {
	if timeLimit == NoTimeLimit {
		return time.Time{}, errors.New("this race has no time limit")
	}

	endTime := startTime.Add(timeLimit)
	return endTime, nil
}

func parseTimeDuration(timeString string) (time.Duration, error) {
	//Parse timer values
	parts := strings.Split(timeString, ":")
	if len(parts) != 3 {
		return NoTimeLimit, errors.New("invalid time string")
	}

	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return NoTimeLimit, errors.New("invalid time string")
	}

	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return NoTimeLimit, errors.New("invalid time string")
	}

	s, err := strconv.Atoi(parts[2])
	if err != nil {
		return NoTimeLimit, errors.New("invalid time string")
	}

	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second, nil
}

//Functions for saving/loading schedule
func (s *raceSchedule) storeSchedule(cachePath string) error {
	s.mu.RLock()
	ctx := s.ctx
	raceID := s.raceID
	startTime := s.startTime
	timeLimit := s.raceLimit
	s.mu.RUnlock()

	//No schedule to save
	if ctx == nil {
		return nil
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheFile, err := os.OpenFile(cachePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	//Get the structure of the store
	c := make(map[string]any)
	c["start_time"] = fmt.Sprint(startTime.Format(time.RFC3339))
	c["time_limit"] = int64(timeLimit)
	c["race_id"] = raceID

	err = json.NewEncoder(cacheFile).Encode(c)
	if err != nil {
		return err
	}

	return nil
}

func (s *raceSchedule) loadSchedule(cachePath string) error {
	//Read info from cache
	cacheMu.RLock()
	c, err := os.ReadFile(cachePath)
	cacheMu.RUnlock()

	if err != nil {
		//File doesn't exist. Not an error, just no saved schedule
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	//Empty cache. Not an error, just no schedule saved
	if len(c) == 0 {
		return nil
	}

	sched := make(map[string]any)
	err = json.Unmarshal(c, &sched)
	if err != nil {
		return err
	}

	scheduledStart, ok := sched["start_time"].(string)
	if !ok {
		return errors.New("unable to get start time from stored schedule. no race has been scheduled.")
	}

	readStart, err := time.Parse(time.RFC3339, scheduledStart)
	if err != nil {
		return err
	}

	timeLimit, ok := sched["time_limit"].(float64)
	if !ok {
		return errors.New("unable to get race time limit from stored schedule. no race has been scheduled")
	}

	totalFinishTime := readStart.Add(time.Duration(timeLimit)).Add(streamEndOffset)
	if totalFinishTime.Before(time.Now()) {
		return errors.New("stored scheduled start time already past. no race has been scheduled")
	}

	//Validate other fields
	raceID, ok := sched["race_id"].(float64)
	if !ok {
		return errors.New("unable to get race id from stored schedule. no race has been scheduled")
	}

	//Schedule race doesn't expect the Z at the end of the time, so split it
	actualStart := strings.Split(fmt.Sprint(readStart.Format(time.RFC3339)), "Z")[0]
	return s.ScheduleRace(int(raceID), actualStart, formatDuration(time.Duration(int64(timeLimit))))
}

func (s *raceSchedule) deleteSchedule(raceID int, cachePath string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	s.mu.RLock()
	currentID := s.raceID
	s.mu.RUnlock()

	//Only delete this race is it hasn't been changed
	if currentID == raceID {
		os.Remove(cachePath)
	}
}

//Helper for turning time duration into string
func formatDuration(d time.Duration) string {
	//Cast to integer seconds to eliminate fractional nanoseconds
	totalSeconds := int64(d.Seconds())

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func (s *raceSchedule) logRace() {
	s.mu.RLock()
	ctx := s.ctx
	raceID := s.raceID
	start := s.startTime
	limit := s.raceLimit
	s.mu.RUnlock()

	if ctx == nil {
		logMessage("No race is scheduled")
		return
	}

	logMsg := fmt.Sprintf("Race %v is scheduled at %s UTC on %s", raceID, start.Format(time.TimeOnly), start.Format(time.DateOnly))
	if limit != NoTimeLimit {
		logMsg += fmt.Sprintf(" with a time limit of %s", limit.String())
	}

	logMessage(logMsg)
}