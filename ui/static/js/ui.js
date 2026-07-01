//File for handling UI elements

//Setup
async function setupUI() {
    //Setup SSE
    const eventSource = new EventSource("/api/events")
    eventSource.onmessage = (event) => {
        logMessage(event.data)
    }
    eventSource.onerror = (e) => console.error("SSE error", e)

    init = async () => {
        selectedRace = await getInProgressRace()
    }
    await init()

    await updateUI()
}

async function updateUI() {
    pageTitle = document.querySelector(".main-ui h1")
    //If we are hosting a race, put the information in the title
    if (!selectedRace) {
        pageTitle.innerHTML = "No Race Selected"
    } else {
        pageTitle.innerHTML = `Hosting ${selectedRace.category} race on ${selectedRace.date} with race ID ${selectedRace.id}`
    }

    //Update the buttons
    await updateButtons()
}

async function updateButtons() {
    //If we are connected to twitch, then change which button is visible
    connected = await connectedToTwitch()
    if (connected) {
        connectToTwitchButton.style.display = "none"
        disconnectFromTwitchButton.style.display = "block"
    } else {
        connectToTwitchButton.style.display = "block"
        disconnectFromTwitchButton.style.display = "none"
    }

    //If no race is selected, you can only select a race
    if (!selectedRace) {
        //Disabled buttons
        beginRaceButton.disabled = true
        resetRaceButton.disabled = true
        endRaceButton.disabled = true

        //Enabled buttons
        selectPastRaceButton.disabled = false
        selectUpcomingRaceButton.disabled = false
        return
    }

    //If a race is in progress, you can't select new races or begin the race. You may finish or reset it.
    if (selectedRace.status === "in_progress") {
        //Disabled buttons
        selectPastRaceButton.disabled = true
        selectUpcomingRaceButton.disabled = true
        beginRaceButton.disabled = true

        //Enabled buttons
        resetRaceButton.disabled = false
        endRaceButton.disabled = false
    }

    //If the selected race is upcoming, you can begin the race, set it to finished, or select a new race
    if (selectedRace.status === "upcoming") {
        //Disabled buttons
        endRaceButton.disabled = true
        resetRaceButton.disabled = true

        //Enabled buttons
        selectPastRaceButton.disabled = false
        selectUpcomingRaceButton.disabled = false
        beginRaceButton.disabled = false
    }

    //If selected race is completed, you can reset the race or select a new race
    if (selectedRace.status === "completed") {
        //Disabled buttons
        endRaceButton.disabled = true
        beginRaceButton.disabled = true

        //Enabld buttons
        selectPastRaceButton.disabled = false
        selectUpcomingRaceButton.disabled = false
        resetRaceButton.disabled = false
    }
}

function showSelectionBox() {
    //Bring up selection box
    var ui = document.querySelector(".main-ui")
    var selectionBox = document.querySelector(".selection-box")
    var submitButton = document.getElementById('selection-submit')
    ui.id = "background"
    selectionBox.style.display = "block"
    submitButton.disabled = true

    //Add cancel event
    document.getElementById("selection-cancel").addEventListener('click', (event) => {
        hideSelectionBox()
    })
}

function hideSelectionBox() {
    var ui = document.querySelector(".main-ui")
    var selectionBox = document.querySelector(".selection-box")
    var selectionForm = document.getElementById("race-select-form")
    ui.id=""
    selectionBox.style.display = "none"
    selectionForm.innerHTML = ''
}