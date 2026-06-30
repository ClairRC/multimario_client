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

    updateUI()
}

function updateUI() {
    pageTitle = document.querySelector(".main-ui h1")
    //If we are hosting a race, put the information in the title
    if (!selectedRace) {
        pageTitle.innerHTML = "No Race Selected"
    } else {
        pageTitle.innerHTML = `Hosting ${selectedRace.category} race on ${selectedRace.date}`
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
}

function hideSelectionBox() {
    var ui = document.querySelector(".main-ui")
    var selectionBox = document.querySelector(".selection-box")
    var selectionForm = document.getElementById("race-select-form")
    ui.id=""
    selectionBox.style.display = "none"
    selectionForm.innerHTML = ''
}