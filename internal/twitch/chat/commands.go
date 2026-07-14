package chat

import (
	"fmt"
	"strconv"
	"strings"

	categoryinfo "github.com/multimario_client/internal/category_info"
	"github.com/multimario_client/internal/store"
)

//Package for handling chat commands

//Maps a command to a function to execute.
//Function returns the response for this command
var chatCommands = make(map[string]func([]string, string) string)

//Adds user as organizer
func commandAddOrganizer(args []string, sender string) string {
	if len(args) != 1 || !store.IsOrganizer(sender) {
		return ""
	}

	store.AddOrganizer(args[0])

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

	targetPlayer := args[0]

	//Update status to Forfeit
	err := store.Race.SetToRunning(targetPlayer)
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

	targetPlayer := args[0]

	//Update status to Forfeit
	err := store.Race.SetPlayerStatus(targetPlayer, "Disqualified")
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

	targetPlayer := args[0]

	//Update status to Forfeit
	err := store.Race.SetPlayerStatus(targetPlayer, "No-show")
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

	targetPlayer := args[0]

	//Update status to Forfeit
	err := store.Race.SetPlayerStatus(targetPlayer, "Forfeit")
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
			targetUser = args[0]
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
			targetUser = args[0]
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

	//Update the player's status
	err := store.Race.SetToRunning(sender)
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

	//Update the player's status
	err := store.Race.SetToQuit(sender)
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

	targetPlayer := strings.ToLower(args[0])
	numToSet, err := strconv.Atoi(args[1])
	if err != nil {
		return "Error adding: Invalid number"
	}

	//Store the update
	_, err = store.Race.SetPlayerCount(numToSet, targetPlayer)
	if err != nil {
		return fmt.Sprintf("Error adding: %s", err.Error())
	}

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
		targetPlayer = args[0]
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
		targetPlayer = args[0]
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
	if len(args) > 2 || len(args) == 0 {
		return ""
	}

	targetPlayer := sender
	newName := args[0]

	if len(args) == 2 {
		if !store.IsOrganizer(sender) {
			return ""
		}
		targetPlayer = args[0]
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

	return fmt.Sprintf("%s now has %v %s in %s.", 
		targetPlayer, categoryinfo.GetGameProgress(raceCat, newNum), 
		categoryinfo.GetCollectibleType(raceCat, newNum), 
		categoryinfo.CurrentGameName(raceCat, newNum))
}

//Adds count based on number of arguments in the command
func commandAdd(args []string, sender string) string {
	//This command only takes at most 2 arguments
	if len(args) > 2 || len(args) == 0 {
		return "" //Do nothing
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

	return fmt.Sprintf("%s now has %v %s in %s.", 
		targetPlayer, categoryinfo.GetGameProgress(raceCat, newNum), 
		categoryinfo.GetCollectibleType(raceCat, newNum), 
		categoryinfo.CurrentGameName(raceCat, newNum))
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
	//If user is on blacklist, do nothing
	if store.IsOnBlacklist(sender) {
		return ""
	}

	args := strings.Split(command, " ")
	comm := args[0]

	if chatCommands[comm] == nil {
		return "Unknown command"
	}

	return chatCommands[comm](args[1:], strings.ToLower(sender))
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
}