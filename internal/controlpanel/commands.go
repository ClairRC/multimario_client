package controlpanel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/multimario_client/internal/obs"
	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch/chat"
)

//This file handles and stores control panel commands
var hostCommands = make(map[string]func(int, []string) (string, error))

func showSchedule(raceID int, args []string) (string, error) {
	return Schedule.getRaceStr(), nil
}

func testOBSConnection(raceID int, args []string) (string, error) {
	if len(args) != 0 {
		return "", errors.New("This command takes no arguments.")
	}

	//Attempt connection
	err := obs.ConnectToOBS()
	defer obs.DisconnectFromOBS()
	if err != nil {
		return "", err
	}

	return "OBS connection successful. Disconnecting...", nil
}

func removeScheduledRaceLimit(raceID int, args []string) (string, error) {
	if len(args) != 0 {
		return "", errors.New("This command takes no arguments")
	}

	err := Schedule.UpdateRaceTimeLimit("")
	if err != nil {
		return "", err
	}

	return "Scheduled race will not end automatically", nil
}

func updateScheduledRaceLimit(raceID int, args []string) (string, error) {
	if len(args) > 1 {
		return "", errors.New("Too many arguments")
	}

	if len(args) < 1 {
		return "", errors.New("Too few arguments")
	}

	err := Schedule.UpdateRaceTimeLimit(args[0])
	if err != nil {
		return "", err
	}

	return "Scheduled race time limit has been updated.", nil
}

func updateScheduledRaceStart(raceID int, args []string) (string, error) {
	if len(args) > 1 {
		return "", errors.New("Too many arguments")
	}

	if len(args) < 1 {
		return "", errors.New("Too few arguments")
	}

	err := Schedule.UpdateStartTime(args[0])
	if err != nil {
		return "", err
	}

	return "Start time has been updated", nil
}

func resetRaceSchedule(raceID int, args []string) (string, error) {
	if len(args) > 0 {
		return "", errors.New("Too many arguments")
	}

	err := Schedule.UnscheduleRace()
	if err != nil {
		return "", err
	}

	return "Race has been unscheduled", nil
}

func setRaceSchedule(raceID int, args []string) (string, error) {
	if len(args) > 2 {
		return "", errors.New("Too many arguments")
	}
	if len(args) < 1 {
		return "", errors.New("Not enough arguments")
	}

	if raceID < 0 {
		return "", errors.New("Unable to schedule race: Invalid race ID")
	}

	//Get values
	startTime := args[0]
	limit := ""
	if len(args) == 2 {
		limit = args[1]
	}

	err := Schedule.ScheduleRace(raceID, startTime, limit)
	if err != nil {
		return "", err
	}

	return "Race has been scheduled", nil
}

func showLog(raceID int, args []string) (string, error) {
	if len(args) > 2 {
		return "", errors.New("Too many arguments")
	}

	numLogs := 100
	if len(args) == 1 {
		localNumLogs, err := strconv.Atoi(args[0])
		if err != nil {
			return "", fmt.Errorf("Unable to parse %s as a number", args[0])
		}
		numLogs = localNumLogs
	}

	//Get logs
	logs, err := chat.GetLog()
	if err != nil {
		return "", fmt.Errorf("Error getting logs: %v", err)
	}

	//Output logs

	startingIndex := max(len(logs) - numLogs, 0)
	recentLogs := logs[startingIndex:]

	logMessage("Logs:\n" + strings.Join(recentLogs, "\n"))

	return "", nil
}

func removeOrganizer(raceID int, args []string) (string, error) {
	if len(args) > 1 {
		return "", errors.New("Too many arguments")
	}

	if len(args) < 1 {
		return "", errors.New("Too few arguments")
	}

	store.RemoveOrganizer(args[0])

	return fmt.Sprintf("%s has been removed from the organizer list.", args[0]), nil
}

func showBlacklist(raceID int, args []string) (string, error) {
	//Get blacklist map
	blacklist := store.Race.GetBlacklist()

	output := "Blacklist: "

	for u := range blacklist {
		output += fmt.Sprintf("%s, ", u)
	}

	return output, nil
}

func showOrganizers(raceID int, args []string) (string, error) {
	//Get organizer map
	organizers := store.Race.GetOrganizerList()

	output := "Organizers: "

	for u := range organizers {
		output += fmt.Sprintf("%s, ", u)
	}

	return output, nil
}

func exportTimes(raceID int, args []string) (string, error) {
	logMessage("Exporting times. This might take a while...")

	err := store.Race.ExportTimes()
	if err != nil {
		return "", err
	}

	return "Times have been exported", nil
}

//Takes a command and a log channel and executes it
func handleCommand(raceID int, command string) error {
	args := strings.Split(command, " ")
	comm := args[0]

	if len(args) > 1 {
		args = args[1:]
	} else {
		args = make([]string, 0)
	}

	//Check this command exists
	if _, exists := hostCommands[comm]; !exists {
		return errors.New("command does not exist")
	}

	response, err := hostCommands[comm](raceID, args)
	if err != nil {
		return err
	}

	if response != "" {
		logMessage(response)
	}

	//Command successful, update the UI
	updateControlPanel()

	return nil
}

func beginSelectedRace(raceID int, args []string) (string, error) {
	startRace()
	return "", nil
}

func initCommands() {
	hostCommands["/exporttimes"] = exportTimes
	hostCommands["/organizers"] = showOrganizers
	hostCommands["/blacklist"] = showBlacklist
	hostCommands["/removeorganizer"] = removeOrganizer
	hostCommands["/showlog"] = showLog
	hostCommands["/schedulerace"] = setRaceSchedule
	hostCommands["/unschedulerace"] = resetRaceSchedule
	hostCommands["/showschedule"] = showSchedule
	hostCommands["/updatescheduledstart"] = updateScheduledRaceStart
	hostCommands["/updatescheduledlimit"] = updateScheduledRaceLimit
	hostCommands["/removescheduledlimit"] = removeScheduledRaceLimit
	hostCommands["/beginrace"] = beginSelectedRace
	hostCommands["/testobs"] = testOBSConnection
}