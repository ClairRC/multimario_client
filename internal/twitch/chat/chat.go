package chat

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/multimario_client/internal/twitch"
)

//Connects to Twitch IRC and joins channel rooms. Returns TLS connection to Twitch IRC channel
func ConnectToChat(userToken string, clientID string, targetRooms []string) error {
	/*
	* TODO: It is probably beneficial to at some point migrate to twitch's EventSub API for chat interfacing rather than IRC
	* It might be worth it to wrap this function in some other function so that the architecture doesn't entirely
	* depend on this TLS connection.
	*/
	conn, err := tls.Dial("tcp", "irc.chat.twitch.tv:6697", &tls.Config{})
	if err != nil {
		return err
	}

	//Get Twitch name from user token
	userName, err := twitch.GetUserNameFromToken(userToken, clientID)
	if err != nil {
		return err
	}

	//Connect to rooms
	fmt.Fprintf(conn, "CAP REQ :twitch.tv/tags twitch.tv/commands\r\n")
	fmt.Fprintf(conn, "PASS oauth:%s\r\n", userToken)
	fmt.Fprintf(conn, "NICK %s\r\n", userName)
	
	for _, name := range targetRooms {
		//TODO: Add a check to make sure that this doesn't fail silently.
		log.Printf("Attempting to connect to %s", name)
		fmt.Fprintf(conn, "JOIN #%s\r\n", name)
		time.Sleep(500 * time.Millisecond) //Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
	}
	
	//Register chat commands
	initCommands()

	//After connecting, listen on this connection and return from this goroutine
	go ListenToChat(conn)

	return nil
}

//Listens to Twitch chat and creates a channel for writing
func ListenToChat(conn *tls.Conn) {
	log.Print("Listening to Twitch chat...")

	//Create write channel and begin goroutine for writes
	writeC := make(chan(string))
	go writeToConnection(conn, writeC)

	//Create scanner for listening
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		//Get message from Twitch chat
		line := scanner.Text()

		//Parse Twitch line in a new goroutine
		go ParseLine(line, writeC)
	}
}

//Takes a line of text and parses it
func ParseLine(line string, responseC chan(string)) {
	//If this is a PING, PONG back
	if strings.HasPrefix(line, "PING") {
		responseC <- fmt.Sprintf("PONG %s\r\n", strings.TrimPrefix(line, "PING "))
		return
	} 

	//Get message and check if its a command. If not, return
	message := extractMessage(line)
	msgIsCommand := isCommand(message)
	if !msgIsCommand {
		return
	}

	//Get message metadata
	metadata := extractMetadata(line)
	senderName := metadata["display-name"]
	channelName := extractChannelName(line)

	//Execute command
	res := executeCommand(message, senderName)
	
	//If there is a response, write it to the same chat
	//Don't write back if it is a command. Probably unnecessary, but don't chain commands
	if res != "" && !isCommand(res) {
		msg := fmt.Sprintf("%s", res)
		writeToTwitchChat(msg, channelName, responseC)
	}
}

//Writes a line to a specified twitch chat
func writeToTwitchChat(message string, channelName string, writeC chan(string)) {
	out := fmt.Sprintf("PRIVMSG #%s :%s", channelName, message)
	writeC <- out
}

//Writes a message to the IRC connection
func writeToConnection(conn *tls.Conn, writeC chan(string)) {
	//TODO: Implement a cooldown to avoid rate limiting
	for {
		out := <-writeC
		fmt.Fprintf(conn, "%s\r\n", out)
	}
}

//Takes a twitch message and extracts the key-value pairs in the message metadata
func extractMetadata(line string) map[string]string {
	out := make(map[string]string)
	message := strings.Split(line, " :")
	metadataStr := message[0]
	metadataArr := strings.Split(metadataStr, ";")

	for _, pair := range metadataArr {
		parts := strings.Split(pair, "=")
		key := parts[0]
		if len(parts) > 1 {
			out[key] = parts[1]
		}
	}

	return out
}

//Takes a twitch message and extracts the message part of it
func extractMessage(line string) string {
	//Splits message into 3 parts. The first two parts aren't necessary, the rest are the message
	messageParts := strings.Split(line, " :")
	message := ""
	for i := 2; i < len(messageParts); i++ {
		if i > 2 {
			message += " :" //Add back on the part that got split for messages that are incorrectly split in the middle
		}
		message += messageParts[i]
	}

	return message
}

//Takes twitch message and returns the channel the message was written in
func extractChannelName(line string) string {
	//TODO: Implement
	return "clairdss"
}