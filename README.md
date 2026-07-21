# Multimario Race Bot
This is a bot for tracking stats for Multimario races written primarily in Go.\
\
The main goals of this bot are decouple the Twitch bot from race state and to allow greater throughput compared to the previously used Twitch bot. 
This means that through a REST API, developers can create tools to update/track race status without going through Twitch, which is something that the current race bot simply doesn't allow.
Additionally, this bot should be less prone to crashing and slow-down compared to the previous bot.\
\
This bot was also made to, hopefully, be more scalable than the previous bot, and to make the source code more readily available.\
\
This bot is heavily inspired by the race bot created by 1upsforlife that the community has been using for over a decade now.

(If you're a racer/organizer looking for the command list, you can find it [here](https://github.com/ClairRC/multimario_client/blob/main/commandlist.md))

# Table of Contents
* [API Documentation](#api-documentation)
* [Installation](#installation)
* [Settings](#settings)
* [Usage](#usage)
* [Known Issues](#known-issues)
* [TODO](#todo)

## API Documentation
The backend's API documentation is still a work in progress. This section will be updated once it is ready. For now, you can view the API's source code [here](https://github.com/ClairRC/multimario_api).

## Installation
To install the bot, you can simply download the resources from the [releases](https://github.com/ClairRC/multimario_client/releases) page and run the executable.

### Building From Source
If you'd like to compile the bot yourself, you need to have Go installed. The bot is cross-platform and can be compiled on any OS. Instructions on that can be found on [Go's official website](https://go.dev/doc/install).\
Once Go is installed, you can run it using `go run <directory>` or compile it using `go build <directory>`. That's all the setup that's required to build the sourcecode 
because Go is magic and will fetch the dependencies for you.

## Settings
To launch the bot, you must first configure the settings in a file called settings.json placed in the same directory as the executable. There is a template settings file provided.

Example:
```json
{
    "twitch_client_id": "Twitch client ID",
    "twitch_client_secret": "Twitch client secret",
    "multimario_api_key": "API Key from backend",
    "multimario_ip": "Multimario backend IP",
    "multimario_port": "Multimario backend Port",
    "obs_ws_password": "OBS Websocket password",
    "layout": "default"
}
```

#### Twitch Client ID
This is the Twitch Client ID that you get from registering your bot with Twitch on the Twitch Developer Console.

#### Twitch Client Secret
This is the Twitch Client Secret that you are provided with, also from registering your bot with Twitch.

#### Multimario API Key
This is the API key for the bot's backend. You can get one by going to https://multimario.app/auth/api_key. This will generate a new key for you that is unique to your Twitch account, so don't share it with others!\
You must have a high enough Auth level to post updates to the backend, so this is currently for organizer use only.

#### Multmario IP
Currently, the backend is hosted at https://multimario.app

#### Multimario Port
443 because HTTPS

#### OBS WS PASSWORD
This is the password for your OBS web socket server. This is not a required field, but without it, if you schedule a race ahead of time, your stream will not automatically begin. Instructions on how to enable OBS websocket server can be found [here](https://obsproject.com/kb/remote-control-guide)\
If you do not provide a websocket password, OBS integration will not be available.

#### Layout
This is the name of the stats page layout you'd like to use. Currently, there is a single layout called "default", which is included in the release and the sourcecode. Layouts are written in vanilla HTML/CSS/JS.

## Usage
The bot hosts a local control panel that gives organizers more control over the race. This page is hosted on `localhost` port `8081`, so to access it you can use `http://localhost:8081`, or if you would like to access it from a different
device on your network, use `http://<your-local-ipv4-address>:8081`.

### Stats Page
The stats page is also hosted localy on port `8080`. To view it, you can use the same methods as above except replace `8081` with `8080`.\
\
Conveniently, this page can also be captured through OBS using OBS browser capture. If you are using this method, make sure that the "custom CSS" field is left blank. The default layout is meant to be 16:9, so make sure to set the dimensions accordingly.

### Selecting a Race
By default, if there is a race in progress, you will be automatically hosting that race upon launching the bot. If there is no race in progress and you're going to host a race, you can 
select either an upcoming race or a past race on the control panel using their respective buttons. Once you select a race, its stats and players will be shown on the stats page.

### Connecting to Chat
By pressing the "Connect To Twitch Chat" button, the bot will automatically try to connect to the chats of every player who is in the selected race. You can connect and disconnect at will.

### Beginning a Race
By pressing the "Begin Race" button, or using the /beginrace command, you will set the currently selected race in progress.\
This has two main side effects: First, this means the /currentrace endpoint on the backend will update to the race you've selected, and 2) The timer on the stats page will be set to 0 and begin counting.

### Resetting/Finishing a Race
When you reset a race, the only thing that happens is the race is set to be "upcoming" and the timer on the stats page will be paused. There is no real functional difference between an upcoming race and a 
finished race, but it's convenient for record-keeping and bot usage. The Finish race button will set the race's status to "completed".

### Commands
There are some convenient commands for the host of the race. These can be found on the [command list](https://github.com/ClairRC/multimario_client/blob/main/commandlist.md).

## Known Issues
- Sometimes, when you select a race, not all of the player cards will be loaded on the stats page. This seems entirely random and is likely due to some race condition. Reloading the page fixes this.
- If you refresh the stats page and the race being hosted has a cached start time, the timer will update to reflect that start time even if it is paused. This isn't so much of a bug as a side effect of caching the start time,
and it shouldn't be an issue since the timer shouldn't be paused during a race anyway, but it's odd behavior nonetheless.

## TODO
- Implement backend polling/websockets to display updated race progress from external API calls.
