//File for functions to communicate with bot client

/*
 * Race objects are formatted as
 * {
 *  category: string
 *  date: string (yyyy-mm-dd)
 *  id: int
 *  status: string (upcoming, in_progress, completed)
 * }
 * 
 * Using JSDocs might be nice here but its nbd for the scope of this little script
*/

//Sends get request to bot to get upcoming races from backend
async function getUpcomingRaces() {
    try {
        const resp = await fetch("/api/upcoming_races")
        if (!resp.ok) {
            throw new Error("HTTP Error: Unable to send request to bot")
        }

        var data = await resp.json()

        return data.races
    } catch (error) {
        logError(error)
    }
}

//Sends get request to bot to get past races from backend
async function getPastRaces() {
    try {
        const resp = await fetch("/api/past_races")
        if (!resp.ok) {
            throw new Error("HTTP Error: Unable to send request to bot")
        }

        var data = await resp.json()

        return data.races
    } catch (error) {
        logError(error)
    }
}

//Returns a json race object or null if no such race exists
async function getInProgressRace() {
    try {
        const resp = await fetch("/api/in_progress_race")
        if (!resp.ok) {
            throw new Error("HTTP Error: Unable to send request to bot")
        }

        //Get response as JSON. If it's empty, return null, otherwise return the object
        var data = await resp.json()

        return data.race
    } catch (error) {
       logError(error)
    }
}

//Sends request to begin the race being hosted
async function beginRace() {
    if (!selectedRace) {
        logError("Unable to begin race: No race selected.")
        return null
    }

    //Send request to begin race
    const response = await fetch(`/api/start_race?race_id=${selectedRace.id}`, {method: 'POST'})

    const result = await response.json()
    if (!result.success) {
        logError(`Unable to begin race: ${result.error}`)
        return
    }

    //No errors, update race status
    selectedRace.status = "in_progress"
}

//Sends request to reset the race being hosted
async function resetRace() {
    if (!selectedRace) {
        logError("Unable to reset race: No race selected.")
        return null
    }

    //Send request to reset race
    const response = await fetch(`/api/reset_race?race_id=${selectedRace.id}`, {method: 'POST'})

    const result = await response.json()
    if (!result.success) {
        logError(`Unable to reset race: ${result.error}`)
        return
    }

    //No errors, update race status
    selectedRace.status = "upcoming"
}

//Sends request to end the race being hosted
async function finishRace() {
    if (!selectedRace) {
        logError("Unable to finish race: No race selected.")
        return null
    }

    //Send request to finish race
    const response = await fetch(`/api/finish_race?race_id=${selectedRace.id}`, {method: 'POST'})

    const result = await response.json()
    if (!result.success) {
        logError(`Unable to finish race: ${result.error}`)
        return
    }

    //No errors, update race status
    selectedRace.status = "completed"
}

async function connectToTwitch() {
    if (!selectedRace) {
        logError("Unable to connect to Twitch chat: No race selected.")
        return null
    }

    //Send request to connect
    const response = await fetch(`/api/connect_to_twitch?race_id=${selectedRace.id}`, {method: 'POST'})

    const result = await response.json()
    if (!result.success) {
        logError(`Unable to connect to Twitch: ${result.error}`)
        return
    }
}

async function disconnectFromTwitch() {
    //Send request to disconnect
    await fetch(`/api/disconnect_from_twitch`, {method: 'POST'})
}

async function connectedToTwitch() {
    //Send request to disconnect
    const response = await fetch(`/api/connected_to_twitch`)
    const result = await response.json()

    return result.connected
}

async function selectRace(raceID) {
    //Send request to connect
    const response = await fetch(`/api/select_race?race_id=${raceID}`, {method: 'POST'})

    const result = await response.json()
    if (!result.success) {
        logError(`Unable to select Race: ${result.error}`)
        return
    }
}