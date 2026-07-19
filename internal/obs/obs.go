package obs

import (
	"errors"
	"fmt"

	"github.com/andreykaipov/goobs"
)

//Package for OBS integration

const NoPassword = ""

var obsWSPass = NoPassword
var obsClient *goobs.Client

func InitOBSPassword(password string) {
	obsWSPass = password
}

//Sets up connection with OBS
func ConnectToOBS() error {
	client, err := goobs.New("localhost:4455", goobs.WithPassword(obsWSPass))
	if err != nil {
		return fmt.Errorf("Unable to connect to OBS: %v", err)
	}
	obsClient = client

	return nil
}

//Tears down connection with OBS
func DisconnectFromOBS() {
	defer obsClient.Disconnect()
	
	if obsClient != nil {
		status, err := obsClient.Stream.GetStreamStatus() 
		if err != nil {
			return
		}

		if status.OutputActive {
			obsClient.Stream.StopStream()
		}

		obsClient = nil
	}
}

//Returns a bool for whether or not OBS is connected
func IsUsingOBS() bool {
	return obsWSPass != NoPassword
}

//Begins stream
func StartStreaming() error {
	if obsClient == nil {
		return errors.New("unable to connect to obs: obs client not initialized.")
	}

	streamStatus, err := obsClient.Stream.GetStreamStatus()
	if err != nil {
		return err
	}

	if !streamStatus.OutputActive {
		_, err := obsClient.Stream.StartStream()
		if err != nil {
			return err
		}
	} else {
		return errors.New("unable to start stream: stream is already active")
	}

	return nil
}

//Ends stream
func EndStreaming() error {
	if obsClient == nil {
		return errors.New("unable to connect to obs: obs client not initialized.")
	}

	streamStatus, err := obsClient.Stream.GetStreamStatus()
	if err != nil {
		return err
	}

	if streamStatus.OutputActive {
		_, err := obsClient.Stream.StopStream()
		if err != nil {
			return err
		}
	} else {
		return errors.New("unable to end stream: stream is not active")
	}

	return nil
}