//Set up button values
const selectUpcomingRaceButton = document.getElementById("select-upcoming-race-btn")
const beginRaceButton = document.getElementById("begin-race-btn")
const resetRaceButton = document.getElementById("reset-race-btn")
const endRaceButton = document.getElementById("end-race-btn")
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
    upcomingRaces.forEach(race => {
        raceMap.set(race.id, race)
        selectionForm.innerHTML += 
            `<label><input type="radio" name="selection" value="${race.id}">Date: ${race.date}<br>Category: ${race.category}<br>Status: ${race.status}</label><br>`
    })

    //Add cancel event
    document.getElementById("selection-cancel").addEventListener('click', (event) => {
        hideSelectionBox()
        return
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
        return
    })
}

async function handleStartRaceButton(event) {

}

async function handleFinishRaceButton(event) {
    
}

//Register button event listeners
selectUpcomingRaceButton.addEventListener('click', handleSelectUpcomingRaceButton)
beginRaceButton.addEventListener('click', handleStartRaceButton)
endRaceButton.addEventListener('click', handleFinishRaceButton)