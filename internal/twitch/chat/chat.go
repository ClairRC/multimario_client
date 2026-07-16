package chat

//File for twitch chat functionality

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/multimario_client/internal/twitch"
)

type TwitchChatClient struct {
	credentials *twitch.TwitchCredentials
	conn *tls.Conn
	writeChannel chan(string)
	cancel context.CancelFunc
	mu sync.RWMutex
}

var Client = TwitchChatClient{}

func (c *TwitchChatClient) SetTwitchConnectionParams(params *twitch.TwitchCredentials) {
	c.credentials = params
}


func (c *TwitchChatClient) IsConnectedToTwitch() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil
}

//Connects to Twitch IRC and joins channel rooms. Returns TLS connection to Twitch IRC channel
func (c *TwitchChatClient) ConnectToChat(targetRooms []string, logChannel chan(string)) {
	/*
	* TODO: It is probably beneficial to at some point migrate to twitch's EventSub API for chat interfacing rather than IRC
	* It might be worth it to wrap this function in some other function so that the architecture doesn't entirely
	* depend on this TLS connection.
	*/

	//If there is already a goroutine with a connection, cancel it
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.mu.Unlock()


	conn, err := tls.Dial("tcp", "irc.chat.twitch.tv:6697", &tls.Config{})
	if err != nil {
		logChannel <- err.Error()
		c.mu.Lock()
		if c.conn == conn {
			cancel()
			c.conn = nil
			c.writeChannel = nil
			c.cancel = nil
		}
		c.mu.Unlock()
		conn.Close()
		return
	}

	//Create write channel for this connection
	writeChannel := make(chan(string), 200)

	//A new connection was started, close this one
	c.mu.Lock()
	select {
	case <-ctx.Done():
		c.mu.Unlock()
		conn.Close()
		return //Return because a newer connection is present
	default:
	}

	c.conn = conn
	c.writeChannel = writeChannel
	c.mu.Unlock()

	//Get Twitch name from user token
	userName, err := twitch.GetUserNameFromToken(c.credentials.UserToken(), c.credentials.ClientID())
	if err != nil {
		logChannel <- err.Error()
		return
	}

	//Write to this connection
	fmt.Fprintf(conn, "CAP REQ :twitch.tv/tags twitch.tv/commands\r\n")
	fmt.Fprintf(conn, "PASS oauth:%s\r\n", c.credentials.UserToken())
	fmt.Fprintf(conn, "NICK %s\r\n", userName)

	//Add logic to make sure we are added to our own twitch chat
	var selfAdded = false

	for _, name := range targetRooms {
		//TODO: Add a check to make sure that this doesn't fail silently.
		if name == userName {
			selfAdded = true
		}

		logChannel <- fmt.Sprintf("Attempting to connect to %s", name)
		fmt.Fprintf(conn, "JOIN #%s\r\n", name)
		time.Sleep(500 * time.Millisecond) //Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
	}

	if !selfAdded {
		logChannel <- fmt.Sprintf("Attempting to connect to %s", userName)
		fmt.Fprintf(conn, "JOIN #%s\r\n", userName)
		time.Sleep(500 * time.Millisecond) //Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
	}
	
	//Register chat commands
	initCommands()

	//After connecting, listen on this connection and return from this goroutine
	go c.ListenToChat(ctx, conn, writeChannel, logChannel)
}

//Listens to Twitch chat and creates a channel for writing
func (c *TwitchChatClient) ListenToChat(ctx context.Context, conn *tls.Conn, writeChannel chan(string), logChannel chan(string)) {
	logChannel <- "Listening to Twitch chat..."

	go c.writeToConnection(ctx, conn, writeChannel)

	//Create scanner for listening
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		//Check if this connection is still current before reading from it
		select {
		case <-ctx.Done():
			return
		default:
		}
		//Get message from Twitch chat
		line := scanner.Text()

		//Parse Twitch line in a new goroutine
		go c.ParseLine(line, writeChannel)
	}
}

//Disconnects from twitch chat
func (c *TwitchChatClient) DisconnectFromChat(logChannel chan(string)) error {
	logChannel <- "Disconnecting from Twitch Chat..."
	c.mu.Lock()
	defer c.mu.Unlock()

	//Nil connection, nothing else to do
	if c.conn == nil {
		return nil
	}

	//Cancel this connection if it exists
	if c.cancel != nil {
		c.cancel()
	}
	err := c.conn.Close()
	c.conn = nil
	c.cancel = nil
	c.writeChannel = nil

	return err
}

//Connects to specific user's chat
func (c *TwitchChatClient) ConnectToUser(twitchRoom string) error {
	c.mu.RLock()
	writeCh, conn := c.writeChannel, c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("bot not currently connected to twitch")
	}

	//Attempt to write to channel if it's open, otherwise return
	select {
	case writeCh <- fmt.Sprintf("JOIN #%s", strings.ToLower(twitchRoom)):
		time.Sleep(500 * time.Millisecond) //Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
		return nil
	default: 
		return errors.New("not connected, or too many messages pending")
	}
}

//Disconnects from specific user's chat
func (c *TwitchChatClient) DisconnectFromUser(twitchRoom string) error {
	c.mu.RLock()
	writeCh, conn := c.writeChannel, c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("bot not currently connected to twitch")
	}

	//Connect to rooms
	select {
	case writeCh <- fmt.Sprintf("PART #%s", strings.ToLower(twitchRoom)):
		//Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
		time.Sleep(500 * time.Millisecond)
		return nil
	default:
		return errors.New("not connected, or too many messages pending")
	}
}

//Takes a line of text and parses it
func (c *TwitchChatClient) ParseLine(line string, writeChannel chan(string)) {
	//If this is a PING, PONG back
	if strings.HasPrefix(line, "PING") {
		select {
		case writeChannel <- fmt.Sprintf("PONG %s\r\n", strings.TrimPrefix(line, "PING ")):
		default:
		}
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
		c.writeToTwitchChat(msg, channelName, writeChannel)
	}
}

//Writes a line to a specified twitch chat
func (c *TwitchChatClient) writeToTwitchChat(message string, channelName string, writeCh chan(string)) {
	out := fmt.Sprintf("PRIVMSG #%s :%s", channelName, message)
	select {
	case writeCh <- out:
	default:
	}
}

//Writes a message to the IRC connection
func (c* TwitchChatClient) writeToConnection(ctx context.Context, conn *tls.Conn, writeChannel chan(string)) {
	//TODO: Implement a cooldown to avoid rate limiting
	for {
		select {
		case <-ctx.Done():
			return //Connection is no longer current, return
		case out := <-writeChannel:
			fmt.Fprintf(conn, "%s\r\n", out)
		}
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
	parts := strings.Split(line, "PRIVMSG #")
	channel := ""
	if len(parts) > 0 {
		channel = strings.Split(parts[1], " :")[0]
	}

	return channel
}