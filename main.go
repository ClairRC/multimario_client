package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/multimario_client/internal/controlpanel"
	"github.com/multimario_client/internal/mmapi"
	"github.com/multimario_client/internal/obs"
	"github.com/multimario_client/internal/stats"
	"github.com/multimario_client/internal/store"
	"github.com/multimario_client/internal/twitch"
	"github.com/multimario_client/internal/twitch/auth"
	"github.com/multimario_client/internal/twitch/chat"
	"golang.org/x/sys/windows"
)

const settingsPath = "settings.json"

type Settings struct {
	TwitchClientID string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
	MMAPIKey string `json:"multimario_api_key"`
	OBSPassword string `json:"obs_ws_password"`
	Layout string `json:"layout"`
	IP string `json:"multimario_ip"`
	Port string `json:"multimario_port"`
}

func main() {
	//Disable Windows terminal's QuickEdit mode
	if runtime.GOOS == "windows" {
		disableQuickEdit()
	}

	//Load settings
	settings, err := loadSettings(settingsPath)
	if err != nil {
		log.Fatalf("unable to load twitch api information from %s: %s", settingsPath, err.Error())
	}
	mmapi.SetMultiMarioAPIParams(settings.IP, settings.Port, settings.MMAPIKey)

	//Get twitch user token
	token, err := auth.GetUserToken(settings.TwitchClientID, settings.TwitchClientSecret)
	if err != nil {
		log.Fatalf("%v", err)
	}

	//Set twitch parameters
	twitch.SetTwitchParams(token, settings.TwitchClientID, settings.TwitchClientSecret)
	chat.Client.SetTwitchConnectionParams(twitch.GetTwitchParams())

	//Set OBS parameters 
	if pass := settings.OBSPassword; pass != "" {
		obs.InitOBSPassword(pass)
	} else {
		log.Printf("no obs websocket password set. not using obs for scheduled races.\n")
	}

	//Check if there's an in progress race and if so store that
	race, err := mmapi.GetInProgressRace()
	if err != nil {
		log.Fatalf("%v", err)
	}

	//Store race if it exists
	if race != nil {
		store.Race.LoadRace(int(race.ID))
	}

	//Initialize control panel
	go controlpanel.InitControlPanel()
	stats.InitStatsPage(settings.Layout)
}

func loadSettings(settingsPath string) (*Settings, error) {
	//Load settings
	settingsFile, err := os.Open(settingsPath)
	if err != nil {
		return nil, err
	}
	defer settingsFile.Close()

	var settings Settings
	err = json.NewDecoder(settingsFile).Decode(&settings)
	if err != nil {
		return nil, err
	}

	return &settings, nil
}

func disableWindowsQuickEdit() error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
    getStdHandle := kernel32.NewProc("GetStdHandle")
    getConsoleMode := kernel32.NewProc("GetConsoleMode")
    setConsoleMode := kernel32.NewProc("SetConsoleMode")

    const stdInputHandle = ^uintptr(10) + 1

    handle, _, _ := getStdHandle.Call(uintptr(0xFFFFFFF6))

    var mode uint32
    _, _, err := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))

    mode &^= 0x0040
    mode |= 0x0080

    ret, _, err := setConsoleMode.Call(handle, uintptr(mode))
    if ret == 0 {
        return err
    }
    return nil
}

func disableQuickEdit() {
	fd := os.Stdin.Fd()
	handle := windows.Handle(fd)

	var mode uint32
	err := windows.GetConsoleMode(handle, &mode)
	if err != nil {
		fmt.Printf("Error getting console mode: %v\n", err)
		return
	}

	mode &^= windows.ENABLE_QUICK_EDIT_MODE
	mode |= windows.ENABLE_EXTENDED_FLAGS

	err = windows.SetConsoleMode(handle, mode)
	if err != nil {
		fmt.Printf("Error setting console mode: %v\n", err)
		return
	}

	fmt.Println("Quick Edit mode has been disabled.")
}
