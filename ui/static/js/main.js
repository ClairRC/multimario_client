//Set up button values
const selectUpcomingRaceButton = document.getElementById("select-upcoming-race-btn")
const selectPastRaceButton = document.getElementById("select-past-race-btn")
const beginRaceButton = document.getElementById("begin-race-btn")
const resetRaceButton = document.getElementById("reset-race-btn")
const endRaceButton = document.getElementById("end-race-btn")
const connectToTwitchButton = document.getElementById("connect-to-twitch-btn")
const disconnectFromTwitchButton = document.getElementById("disconnect-from-twitch-btn")
const submitCommandButton = document.getElementById("submit-cmd-btn")
const commandInputBox = document.getElementById("command-input-box")
const logInfoBox = document.getElementById("log-info-box")

//State information for updating UI.
//Probably shouldn't be global but like for this I think it's fine to keep it like this
var selectedRace = null

setupUI()

//Functions for button events
async function handleSelectUpcomingRaceButton(event) {
    var upcomingRacesPromise = getUpcomingRaces()

    showSelectionBox()

    //Get upcoming races and add them to DOM
    var selectionForm = document.getElementById("race-select-form")
    var raceMap = new Map() //Map for quicker race lookups to store raceid : race obj 
    var upcomingRaces = await upcomingRacesPromise
    upcomingRaces.sort((a, b) => b.date.localeCompare(a.date)) //Sort in reverse chronological order
    upcomingRaces.forEach(race => {
        raceMap.set(race.id, race)
        selectionForm.innerHTML += 
            `<label><input type="radio" name="selection" value="${race.id}">Date: ${race.date}<br>Category: ${race.category}<br>Status: ${race.status}</label><br>`
    })

    //Disable the select button if there's nothing selected
    var radioButtons = document.querySelectorAll('input[name=selection]')
    var selectedRaceOption = -1
    radioButtons.forEach(button => {
        button.checked = false //Uncheck buttons the whenthis pops up
        button.addEventListener('change', (event) => {
            if (!event.target.checked) {
                submitButton.disabled = true
            } else {
                selectedRaceOption = Number(event.target.value) //Update selected race
                submitButton.disabled = false
            }
        })
    })

    //Event for submit button
    var submitButton = document.getElementById('selection-submit')
    submitButton.addEventListener('click', (event) => {
        //Only allow a race to be selected if there isnt one in progress
        if (selectedRace && selectedRace.status == "in_progress") {
            logWarning("Unable to select new race while race is in progress.")
        }
        else {
            selectedRace = raceMap.get(selectedRaceOption)
        }
        updateUI()
        hideSelectionBox()
    }, {once: true})
}

async function handleSelectPastRaceButton(event) {
    var pastRacesPromise = getPastRaces()

    showSelectionBox()

    //Get upcoming races and add them to DOM
    var selectionForm = document.getElementById("race-select-form")
    var raceMap = new Map() //Map for quicker race lookups to store raceid : race obj 
    var pastRaces = await pastRacesPromise
    pastRaces.sort((a, b) => b.date.localeCompare(a.date)) //Sort in reverse chronological order
    pastRaces.forEach(race => {
        raceMap.set(race.id, race)
        selectionForm.innerHTML += 
            `<label><input type="radio" name="selection" value="${race.id}">Date: ${race.date}<br>Category: ${race.category}<br>Status: ${race.status}</label><br>`
    })

    //Disable the select button if there's nothing selected
    var radioButtons = document.querySelectorAll('input[name=selection]')
    var selectedRaceOption = -1
    radioButtons.forEach(button => {
        button.checked = false //Uncheck buttons the whenthis pops up
        button.addEventListener('change', (event) => {
            if (!event.target.checked) {
                submitButton.disabled = true
            } else {
                selectedRaceOption = Number(event.target.value) //Update selected race
                submitButton.disabled = false
            }
        })
    })

    //Event for submit button
    var submitButton = document.getElementById('selection-submit')
    submitButton.addEventListener('click', (event) => {
        //Only allow a race to be selected if there isnt one in progress
        if (selectedRace && selectedRace.status == "in_progress") {
            logWarning("Unable to select new race while race is in progress.")
        }
        else {
            selectedRace = raceMap.get(selectedRaceOption)
        }
        updateUI()
        hideSelectionBox()
    }, {once: true})
}

async function handleStartRaceButton(event) {
    await beginRace()
    updateUI()
}

async function handleFinishRaceButton(event) {
    await finishRace()
    updateUI()
}

async function handleResetRaceButton(event) {
    await resetRace()
    updateUI()
}

async function handleConnectToTwitchButton(event) {
    connectToTwitchButton.disabled = true
    
    try {
        await connectToTwitch()
    } finally {
        connectToTwitchButton.disabled = false
        updateUI()
    }
}

async function handleDisconnectFromTwitchButton(event) {
    await disconnectFromTwitch()
    updateUI()
}

//Register button event listeners
selectUpcomingRaceButton.addEventListener('click', handleSelectUpcomingRaceButton)
selectPastRaceButton.addEventListener('click', handleSelectPastRaceButton)
beginRaceButton.addEventListener('click', handleStartRaceButton)
endRaceButton.addEventListener('click', handleFinishRaceButton)
resetRaceButton.addEventListener('click', handleResetRaceButton)
connectToTwitchButton.addEventListener('click', handleConnectToTwitchButton)
disconnectFromTwitchButton.addEventListener('click', handleDisconnectFromTwitchButton)