package chat

//File for twitch chat functionality

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/multimario_client/internal/twitch"
	"github.com/multimario_client/internal/twitch/auth"
)

type TwitchChatClient struct {
	credentials *twitch.TwitchCredentials
	conn *tls.Conn
	writeChannel chan(string)
	logFunc func(string)
	connectedRooms []string
	cancel context.CancelFunc
	mu sync.RWMutex
	reconnecting atomic.Bool //Atomic bool to make sure reconnecting doesn't have a concurrency issue
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
func (c *TwitchChatClient) ConnectToChat(targetRooms []string, log func(string)) error {
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
		c.conn = nil
		c.writeChannel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.mu.Unlock()


	conn, err := tls.Dial("tcp", "irc.chat.twitch.tv:6697", &tls.Config{})
	if err != nil {
		log(err.Error())
		c.mu.Lock()
		if c.conn == nil {
			cancel()
			c.conn = nil
			c.writeChannel = nil
			c.cancel = nil
		}
		c.mu.Unlock()
		return err
	}

	//Create write channel for this connection
	writeChannel := make(chan(string), 1000)

	//A new connection was started, close this one
	c.mu.Lock()
	select {
	case <-ctx.Done():
		c.mu.Unlock()
		conn.Close()
		return errors.New("newer connection is present")//Return because a newer connection is present
	default:
	}

	c.conn = conn
	c.writeChannel = writeChannel
	c.connectedRooms = targetRooms
	c.logFunc = log
	c.mu.Unlock()

	//Get Twitch name from user token
	userName, err := twitch.GetUserNameFromToken(c.credentials.UserToken(), c.credentials.ClientID())
	if err != nil {
		log(err.Error())
		return err
	}

	//Write to this connection
	fmt.Fprintf(conn, "CAP REQ :twitch.tv/tags twitch.tv/commands\r\n")
	fmt.Fprintf(conn, "PASS oauth:%s\r\n", c.credentials.UserToken())
	fmt.Fprintf(conn, "NICK %s\r\n", userName)

	//Read the response to make sure we're connected
	scanner := bufio.NewScanner(conn)

	//First message is an ACK
	scanner.Scan()

	//Make sure we didn't get an error
	if err := scanner.Err(); err != nil {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
			c.writeChannel = nil
			c.cancel = nil
		}
		c.mu.Unlock()
		cancel()
		conn.Close()
		return err
	}

	if line := scanner.Text(); !strings.HasPrefix(line, ":tmi.twitch.tv CAP * ACK") {
		err := fmt.Errorf("Twitch failed to return ACK. Got %s", line)
		log(err.Error())
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
			c.writeChannel = nil
			c.cancel = nil
		}
		c.mu.Unlock()
		cancel()
		conn.Close()
		return err
	}
	
	//Twitch sends 7 messages with specific prefixes
	expected := []string{":tmi.twitch.tv 001", ":tmi.twitch.tv 002", ":tmi.twitch.tv 003",
		":tmi.twitch.tv 004", ":tmi.twitch.tv 375", ":tmi.twitch.tv 372", ":tmi.twitch.tv 376",
	}

	for _, pref := range expected {
		scanner.Scan()

		//Make sure we don't have an error
		if err := scanner.Err(); err != nil {
			log(err.Error())
			c.mu.Lock()
			if c.conn == conn {
				c.conn = nil
				c.writeChannel = nil
				c.cancel = nil
			}
			c.mu.Unlock()
			cancel()
			conn.Close()
			return err
		}

		if line := scanner.Text(); !strings.HasPrefix(line, pref) {
			err := fmt.Errorf("Twitch returned an unexpected result while connecting.\nExpected: %s\nGot:%s", pref, line)
			log(err.Error())
			c.mu.Lock()
			if c.conn == conn {
				c.conn = nil
				c.writeChannel = nil
				c.cancel = nil
			}
			c.mu.Unlock()
			cancel()
			conn.Close()
			return err
		}
	}

	//Connect to chats
	//Add logic to make sure we are added to our own twitch chat
	var selfAdded = false

	for _, name := range targetRooms {
		if name == userName {
			selfAdded = true
		}

		log(fmt.Sprintf("Attempting to connect to %s", name))
		fmt.Fprintf(conn, "JOIN #%s\r\n", name)
		time.Sleep(500 * time.Millisecond) //Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
	}

	if !selfAdded {
		log(fmt.Sprintf("Attempting to connect to %s", userName))
		fmt.Fprintf(conn, "JOIN #%s\r\n", userName)
		time.Sleep(500 * time.Millisecond) //Sleep for half a second to prevent rate limiting. Twitch allows 20 join attempts per 10 seconds
	}
	
	//Register chat commands
	initCommands()

	//After connecting, listen on this connection and return from this goroutine
	go c.ListenToChat(ctx, conn, scanner, writeChannel, log)

	return nil
}

//Listens to Twitch chat and creates a channel for writing
func (c *TwitchChatClient) ListenToChat(ctx context.Context, conn *tls.Conn, connScanner *bufio.Scanner, writeChannel chan(string), log func(string)) {
	log("Listening to Twitch chat...")

	go c.writeToConnection(ctx, conn, writeChannel)

	//Read scanner for new lines
	for connScanner.Scan() {
		//Check if this connection is still current before reading from it
		select {
		case <-ctx.Done():
			return
		default:
		}
		//Get message from Twitch chat
		line := connScanner.Text()

		//If we get no reads after 7 minutes, assume a timeout
		conn.SetReadDeadline(time.Now().Add(7 * time.Minute))

		//Parse Twitch line in a new goroutine
		go c.ParseLine(line, conn, writeChannel)
	}

	select {
	case <-ctx.Done(): //Closed manually, no problem
		return
	default:
	}

	err := connScanner.Err()
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log("Twitch connection timed out: " + netErr.Error())
		} else {
			log("Twitch connection read error: " + err.Error())
		}
	} else {
		log("Twitch connection closed.")
	}

	//Attempt to reconnect
	go c.handleReconnect()
}

//Disconnects from twitch chat
func (c *TwitchChatClient) DisconnectFromChat(log func(string)) error {
	log("Disconnecting from Twitch Chat...")
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
	
		//Check again that this conection isn't stale and write back the new rooms slic 
		c.mu.Lock()
		if c.conn == nil {
			c.mu.Unlock()
			return errors.New("bot not currently connected to twitch")
		}

		if !slices.Contains(c.connectedRooms, twitchRoom) {
			c.connectedRooms = append(c.connectedRooms, twitchRoom)
		}
		c.mu.Unlock()

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

		//Remove this usre from the connected rooms channel
		c.mu.Lock()
		if c.conn == nil {
			c.mu.Unlock()
			return errors.New("bot not currently connected to twitch")
		}

		if slices.Contains(c.connectedRooms, twitchRoom) {
			for i, v := range c.connectedRooms {
				if v == twitchRoom {
					c.connectedRooms[i] = c.connectedRooms[len(c.connectedRooms)-1]
					c.connectedRooms[len(c.connectedRooms)-1] = v
					c.connectedRooms = c.connectedRooms[:len(c.connectedRooms)-1]
					break
				}
			}
		}
		c.mu.Unlock()

		return nil
	default:
		return errors.New("not connected, or too many messages pending")
	}
}

//Takes a line of text and parses it
func (c *TwitchChatClient) ParseLine(line string, conn *tls.Conn, writeChannel chan(string)) {
	//If this is a PING, PONG back
	if strings.HasPrefix(line, "PING") {
		c.writePONG(conn, line)
		return
	} 

	//Twitch is shutting down our connection and we need to reconnect
	if strings.HasPrefix(line, ":tmi.twitch.tv RECONNECT") {
		fmt.Printf("Twitch terminated this connection: %s. Reconnecting...", line)
		c.handleReconnect()
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

func (c* TwitchChatClient) writePONG(conn *tls.Conn, line  string) {
	c.mu.Lock()
	fmt.Fprintf(conn, "PONG %s\r\n", strings.TrimPrefix(line, "PING "))
	c.mu.Unlock()
}

//Writes a message to the IRC connection
func (c* TwitchChatClient) writeToConnection(ctx context.Context, conn *tls.Conn, writeChannel chan(string)) {
	limit := 19
	timeout := 30 * time.Second
	s := make(chan(struct{}), limit)

	for {
		select {
		case <-ctx.Done():
			return //Connection is no longer current, return
		case out := <-writeChannel:
			//If this is a chat message, block if the semaphore is at its limit
			if strings.HasPrefix(out, "PRIVMSG") {
				select {
				case s <- struct{}{}:
				case <-ctx.Done():
					return
				}

				//Set timeout to drain the channel
				go func() {
					<-time.After(timeout)
					<-s
				}()
			}
			c.mu.Lock()
			fmt.Fprintf(conn, "%s\r\n", out)
			c.mu.Unlock()
		}
	}
}

//Reconnects to Twitch after we've died
func (c* TwitchChatClient) handleReconnect() {
	if !c.reconnecting.CompareAndSwap(false, true) {
		return
	}
	defer c.reconnecting.Store(false)
	
	//Get connection information to reconnect
	c.mu.Lock()
	rooms := slices.Clone(c.connectedRooms)
	logFunc := c.logFunc

	//Attempt to refresh the current token before reconnecting to ensure it's current
	newToken, err := auth.RefreshExpiredToken(c.credentials.ClientID(), c.credentials.ClientSecret())
	if err != nil {
		c.mu.Unlock()
		c.logFunc(fmt.Sprintf("Unable to reconnect to Twitch: %s\nConnection will be closed.", err.Error()))
		return
	}

	c.credentials.SetNewUserToken(newToken)
	c.mu.Unlock()

	err = c.ConnectToChat(rooms, logFunc)

	maxReconnects := 10
	for i := 0; err != nil && i < maxReconnects; i++ {
		//Wait 30 seconds before retrying
		c.logFunc(fmt.Sprintf("Unable to reconnect to Twitch: %s. Trying again...", err.Error()))
		time.Sleep(30 * time.Second)
		err = c.ConnectToChat(rooms, logFunc)
	}

	if err != nil {
		c.logFunc(fmt.Sprintf("Can't reconnect to Twitch: %s.", err.Error()))
	} else {
		c.logFunc("Reconnection successful.")
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
	if len(parts) > 1 {
		channel = strings.Split(parts[1], " :")[0]
	}

	return channel
}