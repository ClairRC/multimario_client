package controlpanel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch/chat"
)

//This file handles and stores control panel commands
var hostCommands = make(map[string]func([]string, chan(string)) (string, error))

func showLog(args []string, logCh chan(string)) (string, error) {
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
	logCh <- "Logs: "
	startingIndex := max(len(logs) - numLogs, 0)

	for i := startingIndex; i < len(logs); i++ {
		logCh <- logs[i]
	}

	return "", nil
}

func removeOrganizer(args []string, logCh chan(string)) (string, error) {
	if len(args) > 1 {
		return "", errors.New("Too many arguments")
	}

	if len(args) < 1 {
		return "", errors.New("Too few arguments")
	}

	store.RemoveOrganizer(args[0])

	return fmt.Sprintf("%s has been removed from the organizer list.", args[0]), nil
}

func showBlacklist(args []string, logCh chan(string)) (string, error) {
	//Get blacklist map
	blacklist := store.Race.GetBlacklist()

	output := "Blacklist: "

	for u := range blacklist {
		output += fmt.Sprintf("%s, ", u)
	}

	return output, nil
}

func showOrganizers(args []string, logCh chan(string)) (string, error) {
	//Get organizer map
	organizers := store.Race.GetOrganizerList()

	output := "Organizers: "

	for u := range organizers {
		output += fmt.Sprintf("%s, ", u)
	}

	return output, nil
}

func exportTimes(args []string, logCh chan(string)) (string, error) {
	select {
	case logCh <- "Exporting times. This might take a while...":
	default:
	}

	err := store.Race.ExportTimes()
	if err != nil {
		return "", err
	}

	return "Times have been exported", nil
}

//Takes a command and a log channel and executes it
func handleCommand(command string, logCh chan(string)) error {
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

	response, err := hostCommands[comm](args, logCh)
	if err != nil {
		return err
	}

	if response != "" {
		select {
		case logCh <- response:
		default:
		}
	}

	return nil
}

func initCommands() {
	hostCommands["/exporttimes"] = exportTimes
	hostCommands["/organizers"] = showOrganizers
	hostCommands["/blacklist"] = showBlacklist
	hostCommands["/removeorganizer"] = removeOrganizer
	hostCommands["/showlog"] = showLog
}