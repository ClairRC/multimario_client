package chat

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	categoryinfo "github.com/multimario_client/internal/category_info"
	"github.com/multimario_client/internal/store"
)

//Package for handling chat commands

//Maps a command to a function to execute.
//Function returns the response for this command
var chatCommands = make(map[string]func([]string, string) string)
var commandListURL = "https://github.com/ClairRC/multimario_client/blob/main/commandlist.md"

var cmdLogPath = "./commands.log"
var maxLogSize = 1000
var logMu sync.RWMutex

func commandShowPlacement(args []string, sender string) string {
	if len(args) != 1 {
		return ""
	}

	//Convert this argument to an int
	placement, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Sprintf("Unable to parse %s as a number.", args[0])
	}

	//Get players at this placement
	players, err := store.Race.GetPlayersAtPlacement(placement)
	if err != nil {
		return err.Error()
	}

	maxPlayerCount := 5
	if len(players) == 0 {
		return fmt.Sprintf("No players in place %v", placement)
	}

	outMsg := fmt.Sprintf("Players in place #%v: ", placement)
	for i, p := range players {
		if i >= maxPlayerCount {
			outMsg += "... (Too many players to list)."
			break
		}

		if i == 0 {
			outMsg += p
		} else {
			outMsg += fmt.Sprintf(", %s", p)
		}
	}

	return outMsg
}

//Removes a user from the whitelist
func commandUnwhitelistUser(args []string, sender string) string {
	//Only organizers can unwhitelist people
	if len(args) != 1 || !store.IsOrganizer(sender) {
		return ""
	}

	if !store.IsOnWhitelist(strings.ToLower(args[0])) {
		return fmt.Sprintf("%s isn't whitelisted", args[0])
	}

	store.RemoveWhitelistUser(args[0])
	return fmt.Sprintf("%s has been unwhitelisted", args[0])
}

//Adds a player to the whitelist
func commandWhitelistUser(args []string, sender string) string {
	//Only allowed to whitelist if the sender IS on the whitelist and the target is NOT on the blacklist
	if len(args) != 1 || !store.IsOnWhitelist(sender) || store.IsOnBlacklist(args[0]) {
		return ""
	}

	if store.IsOnWhitelist(strings.ToLower(args[0])) {
		return fmt.Sprintf("%s is already white listed", args[0])
	}

	store.AddWhitelistUser(strings.ToLower(args[0]))
	return fmt.Sprintf("%s has been added to the whitelist", args[0])
}

//Removes user from blacklist
func commandUnblacklistUser(args []string, sender string) string {
	if len(args) != 1 || !store.IsOrganizer(sender) || store.IsOrganizer(args[0]){
		return ""
	}

	store.RemoveBlacklistUser(strings.ToLower(args[0]))

	return args[0] + " has been un-blacklisted"
}

//Adds user to blacklist
func commandBlacklistUser(args []string, sender string) string {
	if len(args) != 1 || !store.IsOrganizer(sender) || store.IsOrganizer(args[0]) {
		return ""
	}

	store.AddBlacklistUser(strings.ToLower(args[0]))

	return args[0] + " has been blacklisted"
}

//Posts command list
func commandMMHelp(args []string, sender string) string {
	if len(args) != 0 {
		return ""
	}

	return fmt.Sprintf("Command list: %s", commandListURL)
}

//Adds user as organizer
func commandAddOrganizer(args []string, sender string) string {
	if len(args) != 1 || !store.IsOrganizer(sender) || store.IsOnBlacklist(args[0]){
		return ""
	}

	store.AddOrganizer(strings.ToLower(args[0]))

	return args[0] + " has been added as an organizer"
}

//Stops the timer
func commandStartTimer(args []string, sender string) string {
	if len(args) != 0 || !store.IsOrganizer(sender) {
		return ""
	}

	store.Race.StartTimer()
	return "Timer has been started"
}

//Stops the timer
func commandStopTimer(args []string, sender string) string {
	if len(args) != 0 || !store.IsOrganizer(sender) {
		return ""
	}

	store.Race.StopTimer()
	return "Timer has been stopped"
}

//Sets current timer value
func commandSetTimer(args []string, sender string) string {
	if len(args) != 1 || !store.IsOrganizer(sender) {
		return ""
	}

	newTime := args[0]

	err := store.Race.SetTimerValue(newTime)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return "Timer has been updated"
}

//Sets player back to running. Only used by admins
func commandRevive(args []string, sender string) string {
	//Only takes 1 argument
	if len(args) != 1 || !store.IsOrganizer(sender){
		return ""
	}

	targetPlayer := strings.ToLower(args[0])

	//Only allowed to quit if player isn't already quit
	status, err := store.Race.GetPlayerStatus(targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	if status == "running" {
		return fmt.Sprintf("Cannot unquit, %s is still in the race.", targetPlayer)
	}

	//Update status to Forfeit
	err = store.Race.SetPlayerStatus(targetPlayer, "running")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return targetPlayer + " has rejoined the race."
}

//Sets player as disqualified. Only used by admins
func commandDQ(args []string, sender string) string {
	//Only takes 1 argument
	if len(args) != 1 || !store.IsOrganizer(sender) {
		return ""
	}

	targetPlayer := strings.ToLower(args[0])

	//Only allowed to quit if player isn't already finished
	finished, err := store.Race.PlayerIsFinished(targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	if finished {
		return fmt.Sprintf("Cannot disqualify, %s is already finished.", targetPlayer)
	}

	//Update status to Forfeit
	err = store.Race.SetPlayerStatus(targetPlayer, "Disqualified")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return ""
}

//Sets player as noshow. Only used by admins
func commandNoShow(args []string, sender string) string {
	//Only takes 1 argument
	if len(args) != 1 || !store.IsOrganizer(sender) {
		return ""
	}

	targetPlayer := strings.ToLower(args[0])

	//Only allowed to quit if player isn't already finished
	finished, err := store.Race.PlayerIsFinished(targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	if finished {
		return fmt.Sprintf("Cannot set player to no-show, %s is already finished.", targetPlayer)
	}

	//Update status to Forfeit
	err = store.Race.SetPlayerStatus(targetPlayer, "No-show")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return ""
}

//Forces a player to quit. Only used by admins
func commandForceQuit(args []string, sender string) string {
	//Only takes 1 argument
	if len(args) != 1 || !store.IsOrganizer(sender) {
		return ""
	}

	targetPlayer := strings.ToLower(args[0])

	//Only allowed to quit if player isn't already finished
	finished, err := store.Race.PlayerIsFinished(targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	if finished {
		return fmt.Sprintf("Cannot quit, %s is already finished.", targetPlayer)
	}

	//Update status to Forfeit
	err = store.Race.SetPlayerStatus(targetPlayer, "Forfeit")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return targetPlayer + " has forfeit!"
}

//Gets the bot to attempt to leave your chat
func commandMMLeave(args []string, sender string) string {
	if len(args) > 1 {
		return ""
	}

	targetUser := sender
	
	//Target user can only be specified if we're an admin
	if len(args) == 1 {
		if !store.IsOrganizer(sender) {
			return ""
		} else {
			targetUser = strings.ToLower(args[0])
		}
	}

	err := Client.DisconnectFromUser(targetUser)
	if err != nil {
		return fmt.Sprintf("Error disconnecting: %s", err.Error())
	}
	return "Disconnected from " + targetUser
}

//Gets the bot to attempt to join your chat
func commandMMJoin(args []string, sender string) string {
	if len(args) > 1 {
		return ""
	}

	targetUser := sender
	
	//Target user can only be specified if we're an admin
	if len(args) == 1 {
		if !store.IsOrganizer(sender) {
			return ""
		} else {
			targetUser = strings.ToLower(args[0])
		}
	}

	err := Client.ConnectToUser(targetUser)
	if err != nil {
		return fmt.Sprintf("Error connecting: %s", err.Error())
	}
	return "Connected to " + targetUser
}

//Unquits
func commandUnquit(args []string, sender string) string {
	if len(args) != 0 {
		return ""
	}

	//Only allowed to quit if player isn't already quit
	status, err := store.Race.GetPlayerStatus(sender)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	if status == "running" {
		return fmt.Sprintf("Cannot unquit, %s is still in the race.", sender)
	}

	//Update the player's status
	err = store.Race.SetPlayerStatus(sender, "running")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return fmt.Sprintf("%s has rejoined the race.", sender)
}

//Sets the player status to be forfeit
func commandQuit(args []string, sender string) string {
	//No arguments allowed
	if len(args) != 0 {
		return ""
	}

	//Only allowed to quit if player isn't already finished
	finished, err := store.Race.PlayerIsFinished(sender)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	if finished {
		return fmt.Sprintf("Cannot quit, %s is already finished.", sender)
	}

	//Update the player's status
	err = store.Race.SetPlayerStatus(sender, "Quit")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return fmt.Sprintf("%s has forfeit!", sender)
}

//Same as set but doesn't send back a confirmation message.
//This is only included for legacy support for counting bots
func commandBotSet(args []string, sender string) string {
	//This command takes 2 arguments
	if len(args) != 2 {
		return "" //Do nothing
	}

	//Only allow whitelisted users to do this
	if !store.IsOnWhitelist(sender) {
		return "Cannot update player count: User is not whitelisted"
	}

	targetPlayer := strings.ToLower(args[0])
	numToSet, err := strconv.Atoi(args[1])
	if err != nil {
		return "Error adding: Invalid number"
	}

	//Store the update
	newNum, err := store.Race.SetPlayerCount(numToSet, targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error adding: %s", err.Error())
	}

	//Log this update if there is a race in progress
	go store.Race.LogPlayerState(targetPlayer, newNum)

	//No output, again for legacy bot support
	return ""
}

func commandSetFinalTime(args []string, sender string) string {
	//Accepts 1 arg or two if the sender is a admin
	if len(args) > 2 || len(args) == 0 {
		return ""
	}

	targetPlayer := sender
	newTime := args[0]
	if len(args) == 2 {
		//Only organizers can set someone else's time
		if !store.IsOrganizer(sender) {
			return ""
		}
		targetPlayer = strings.ToLower(args[0])
		newTime = args[1]
	}

	err := store.Race.UpdateFinalTime(targetPlayer, newTime)
	if err != nil {
		return fmt.Sprintf("Error updating time: %s", err.Error())
	}

	return fmt.Sprintf("Final time for %s has been set to %s.", targetPlayer, newTime)
}

func commandSetGameTime(args []string, sender string) string {
	//Accepts up to 3 args
	if len(args) > 3 || len(args) < 2 {
		return ""
	}

	targetPlayer := sender
	gameName := args[0]
	newTime := args[1]

	if len(args) == 3 {
		if !store.IsOrganizer(sender) {
			return ""
		}
		targetPlayer = strings.ToLower(args[0])
		gameName = args[1]
		newTime = args[2]
	}

	err := store.Race.UpdateGameTime(targetPlayer, gameName, newTime)
	if err != nil {
		return fmt.Sprintf("Error updating time: %s", err.Error())
	}

	return fmt.Sprintf("%s time for %s has been set to %s.", gameName, targetPlayer, newTime)
}

//Sets player's onscreen name
func commandSetName(args []string, sender string) string {
	if len(args) > 2 || len(args) == 0 || !store.IsOrganizer(sender) {
		return ""
	}

	targetPlayer := sender
	newName := args[0]

	if len(args) == 2 {
		if !store.IsOrganizer(sender) {
			return ""
		}
		targetPlayer = strings.ToLower(args[0])
		newName = args[1]
	}

	err := store.Race.UpdatePlayerName(targetPlayer, newName)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	return "Player display name has been updated."
}

//Sets player's count
func commandSet(args []string, sender string) string {
	//This command only takes at most 2 arguments
	if len(args) > 2 || len(args) == 0 {
		return "" //Do nothing
	}

	//Only allow whitelisted users to do this
	if !store.IsOnWhitelist(sender) {
		return "Cannot update player count: User is not whitelisted"
	}

	var targetPlayer string
	var numToSet int
	if len(args) == 1 {
		targetPlayer = sender
		localNumToSet, err := strconv.Atoi(args[0])
		if err != nil {
			return "Error adding: Invalid number"
		}

		numToSet = localNumToSet
	}

	if len(args) == 2 {
		targetPlayer = strings.ToLower(args[0])
		localNumToSet, err := strconv.Atoi(args[1])
		if err != nil {
			return "Error adding: Invalid number"
		}

		numToSet = localNumToSet
	}

	//Store the update
	newNum, err := store.Race.SetPlayerCount(numToSet, targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error adding: %s", err.Error())
	}

	raceCat, err := store.Race.GetCurrentRaceCategory()
	if err != nil {
		return fmt.Sprintf("Error adding: %s", err.Error())
	}

	//Log this update if there is a race in progress
	go store.Race.LogPlayerState(targetPlayer, newNum)

	//Get output
	outMessage := fmt.Sprintf("%s now has %v %s in %s.", 
		targetPlayer, categoryinfo.GetGameProgress(raceCat, newNum), 
		categoryinfo.GetCollectibleType(raceCat, newNum), 
		categoryinfo.CurrentGameName(raceCat, newNum))

	//Get this player's placement
	placement, err := store.Race.GetPlayerPlacement(targetPlayer)

	//No error, add placement to output message
	if err == nil {
		outMessage = fmt.Sprintf("%s (%s Place)", outMessage, placement)
	}

	return outMessage
}

//Adds count based on number of arguments in the command
func commandAdd(args []string, sender string) string {
	//This command only takes at most 2 arguments
	if len(args) > 2 || len(args) == 0 {
		return "" //Do nothing
	}

	//Only allow whitelisted users to do this
	if !store.IsOnWhitelist(sender) {
		return "Cannot update player count: User is not whitelisted"
	}

	var targetPlayer string
	var numToAdd int
	if len(args) == 1 {
		targetPlayer = sender
		localNumToAdd, err := strconv.Atoi(args[0])
		if err != nil {
			return "Error adding: Invalid number"
		}

		numToAdd = localNumToAdd
	}

	if len(args) == 2 {
		targetPlayer = strings.ToLower(args[0])
		localNumToAdd, err := strconv.Atoi(args[1])
		if err != nil {
			return "Error adding: Invalid number"
		}

		numToAdd = localNumToAdd
	}

	//Store the update
	newNum, err := store.Race.AddToPlayerCount(numToAdd, targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error adding: %s", err.Error())
	}

	raceCat, err := store.Race.GetCurrentRaceCategory()
	if err != nil {
		return fmt.Sprintf("Error adding: %s", err.Error())
	}

	//Log this update if there is a race in progress
	go store.Race.LogPlayerState(targetPlayer, newNum)

	//Get output
	outMessage := fmt.Sprintf("%s now has %v %s in %s.", 
		targetPlayer, categoryinfo.GetGameProgress(raceCat, newNum), 
		categoryinfo.GetCollectibleType(raceCat, newNum), 
		categoryinfo.CurrentGameName(raceCat, newNum))

	//Get this player's placement
	placement, err := store.Race.GetPlayerPlacement(targetPlayer)

	//No error, add placement to output message
	if err == nil {
		outMessage = fmt.Sprintf("%s (%s Place)", outMessage, placement)
	}

	return outMessage
}

//Takes a string and checks if it is a command for this bot
func isCommand(line string) bool {
	//Get the first part of the message
	msg := strings.Split(line, " ")
	if len(msg) == 0 {
		return false //Shouldn't happen but would rather not panic
	}
	cmd := msg[0]
	if chatCommands[cmd] != nil {
		return true
	}
	
	return false
}

//Wrapper for executing commands
func executeCommand(command string, sender string) string {
	//Log this command
	go logCommand(fmt.Sprintf("%s: %s", sender, command), cmdLogPath)

	sender = strings.ToLower(sender) //just to make sure that this is lowercase 

	//If user is on blacklist, do nothing
	if store.IsOnBlacklist(sender) {
		return ""
	}

	args := strings.Split(command, " ")
	comm := strings.ToLower(args[0])

	if chatCommands[comm] == nil {
		return "Unknown command"
	}

	return chatCommands[comm](args[1:], sender)
}

//Logs command
func logCommand(command string, filePath string) {
	logMu.Lock()
	defer logMu.Unlock()

	//Open file
	logFile, err := os.Open(filePath)
	fileExists := true
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error opening log file: %s\n", err.Error())
			return
		} else {
			fileExists = false
		}
	}

	//Read file and get final index
	lines := make([]string, 0)
	if fileExists {
		scanner := bufio.NewScanner(logFile)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		//Determine if we need to cut off the oldest command
		if len(lines) >= maxLogSize {
			lines = lines[(len(lines)-maxLogSize+1):] //Cut off the extra logs
		}

		//Open file for writing and then write back to the file
		logFile.Close()

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading from log file: %s\n", err.Error())
			return
		}
	}

	//Append the newest log value
	lines = append(lines, command)

	//Write back to log file
	logFile, err = os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %s\n", err.Error())
		return
	}
	defer logFile.Close()

	for _, l := range lines {
		b := []byte(fmt.Sprintf("%s\n", l))
		logFile.Write(b)
	}
}

//Returns the contents of the log file as a string slice
func GetLog() ([]string, error) {
	logMu.RLock()
	defer logMu.RUnlock()

	//Attempt to open file
	logFile, err := os.Open(cmdLogPath)
	if err != nil {
		return nil, fmt.Errorf("Error opening log file: %v", err)
	}
	defer logFile.Close()

	//Read values
	scanner := bufio.NewScanner(logFile)
	out := make([]string, 0)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading from log file: %v", err)
	}

	return out, nil
}

//Populates commands map with chat commands
func initCommands() {
	chatCommands["!add"] = commandAdd
	chatCommands["!set"] = commandSet
	chatCommands["!setname"] = commandSetName
	chatCommands["!setgametime"] = commandSetGameTime
	chatCommands["!setfinaltime"] = commandSetFinalTime
	chatCommands["!botset"] = commandBotSet
	chatCommands["!quit"] = commandQuit
	chatCommands["!unquit"] = commandUnquit
	chatCommands["!rejoin"] = commandUnquit
	chatCommands["!mmjoin"] = commandMMJoin
	chatCommands["!mmleave"] = commandMMLeave
	chatCommands["!forcequit"] = commandForceQuit
	chatCommands["!noshow"] = commandNoShow
	chatCommands["!dq"] = commandDQ
	chatCommands["!revive"] = commandRevive
	chatCommands["!settimer"] = commandSetTimer
	chatCommands["!stoptimer"] = commandStopTimer
	chatCommands["!starttimer"] = commandStartTimer
	chatCommands["!addorganizer"] = commandAddOrganizer
	chatCommands["!mmhelp"] = commandMMHelp
	chatCommands["!blacklist"] = commandBlacklistUser
	chatCommands["!unblacklist"] = commandUnblacklistUser
	chatCommands["!whitelist"] = commandWhitelistUser
	chatCommands["!unwhitelist"] = commandUnwhitelistUser
	chatCommands["!place"] = commandShowPlacement
}