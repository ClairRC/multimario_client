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