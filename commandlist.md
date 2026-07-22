# Twitch Command List
## Everyone
### !mmhelp
- Sends the link to the command list

## Racers and Counters
### !add [twitch_name] [amount]
- Adds [amount] to [twitch_name]'s score
- [twitch_name] is optional for racers
- Positive and negative numbers are allowed

### !set [twitch_name] [amount]
- Sets [twitch_name]'s score to [amount]
- [twitch_name] is optional for racers

### !botset [twitch_name] [amount]
- Same as !set. This is for legacy support for counting bots.
- [twitch_name] is not optional.

## Racers
### !setname [new_name]
- Sets your name on stats stream to [new_name]
- Still have to use Twitch name for commands

### !setgametime [sm64|smg1|sms|smg2|smo|sm3dw] [hh:mm:ss]
- Sets your finish time for a specific game

### !setfinaltime [hh:mm:ss]
- Sets your final time for the race

### !quit
- Set yourself to "Forfeit"

### !unquit/!rejoin
- Un-quits

### !mmjoin
- Asks the bot to join your Twitch chat

### !mmleave
- Asks the bot to leave your Twitch chat

## Organizers
### !setname [twitch_name] [display_name]
- Sets the name of another user

### !setgametime [twitch_name] [sm64|smg1|sms|smg2|smo|sm3dw] [hh:mm:ss]
- Sets individual game time for another user

### !setfinaltime [twitch_name] [hh:mm:ss]
- Sets final time for another user

### !mmjoin [twitch_name]
- Asks the bot to join the chat of [twitch_name]

### !mmleave [twitch_name]
- Asks the bot to leave the chat of [twitch_name]

### !forcequit [twitch_name]
- Sets another user to "Forfeit"

### !noshow [twitch_name]
- Sets another user to "No-show"

### !dq [twitch_name]
- Sets another user to "Disqualified"

### !revive [twitch_name]
- Puts the user back in the race

### !settimer [hh:mm:ss]
- Sets the on-stream timer to the value given
- IMPORTANT: If the race is scheduled to end at a certain time, setting the timer past that time will automatically end the race.\
Organizers should be very careful with this command and only use it to ensure the timer is accurate!!!

### !stoptimer
- Pauses the on-stream timer

### !starttimer
- Resumes the on-stream timer

### !blacklist [twitch_name]
- Adds user to blacklist, preventing them from using any commands

### !unblacklist [twitch_name]
- Removes user from blacklist

### !addorganizer [twitch_name]
- Adds a user as an organizer, allowing them to use the commands in this section.

# Host Command List
## Used by the race host in the control panel
### /exporttimes
- Exports the individual times and final time for each player in the selected race.
- Can't be used on a race that's in progress

### /organizers
- Shows the list of organizers

### /blacklist
- Shows the blacklist

### /removeorganizer [twitch_name]
- Removes the specified user as organizer

### /showlog [num_logs]
- Shows the most recent [num_logs] logs
- [num_logs] is optional, defaults to 100
- Most recent logs are shown lower in the terminal

### /schedulerace [race_start] [race_limit]
- Schedules a race to automatically begin at [race_start] and automatically end after [race_limit]
- [race_start] should be formatted using RFC3339 format. Examply: YYYY-MM-DDTHH:MM:SS. Time must be in UTC and this format must be followed exactly.
- [race_limit] must be formatted as [hh:mm:ss]
- [race_limit] is optional.
- You can only have 1 race scheduled at a time.
- If there is no OBS websocket password configured, the race will begin without streaming.
- The stream will begin and bot will connect to chat 30 minutes before the race is scheduled to begin.
- Scheduled races are remembered even if the bot is re-opened, but it should at least be opened when the race starts.
- If the bot is opened when a scheduled race is in progress, it will still begin streaming/connect to chat/start the race until the race is scheduled to end.

### /unschedulerace
- Unschedules any scheduled events.
- If this command is called during a race, the race/stream will not automatically end and the bot will not automatically disconnect from chat until the bot is closed.

### /showschedule
- Shows the scheduled race information.

### /updatescheduledstart [new_start]
- Updates the scheduled start time.
- Uses same format as /schedulerace (YYYY-MM-DDTHH:MM:SS UTC time)

### /updatescheduledlimit [new_limit]
- Updates the race time limit. hh:mm:ss format
- This command will only work up until the race begins, so make sure the time limit is correct before then.

### /removescheduledlimit
- Sets the scheduled race to no longer have a time limit.

### /beginrace
- Same functionality as pressing the "Begin Race" button.

### /testobs
- Tests to make sure that the bot is connected to OBS via websockets.
