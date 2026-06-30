package chat

import (
	"fmt"
	"strings"
)

//Package for handling chat commands

//Maps a command to a function to execute.
//Function returns the response for this command
var chatCommands = make(map[string]func([]string, string) string)

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
	args := strings.Split(command, " ")
	comm := args[0]

	if chatCommands[comm] == nil {
		return "Unknown command"
	}

	return chatCommands[comm](args[1:], sender)
}

//Adds count based on number of arguments in the command
func commandAdd(args []string, sender string) string {
	//TODO: Implement properly
	res := ""
	for _, arg := range args {
		res += fmt.Sprintf("%s ", arg)
	}
	return "yup, heard"
}

//Populates commands map with chat commands
func initCommands() {
	chatCommands["!add"] = commandAdd
}