//File for timer stuff

var timerElem = document.querySelector(".timer")
var timerRunning = false
var startTime = undefined
var currentTimerString = "00:00:00"

export function timerUpdate(data) {
    var newTimerVal = "00:00:00"

    if (data.timer_value !== undefined) {
        newTimerVal = data.timer_value

        //Update timer visually
        timerElem.innerHTML = data.timer_value
    }

    if (data.timer_running !== undefined) {
        timerRunning = data.timer_running
    }

    if (timerRunning) {
        startTimer(timerElem.innerHTML)
    }
}

//Checks to make sure the passed in timer value is valid
export function timerValueIsValid(timerValue) {
    //Split the string
    const individualVals = timerValue.split(":")
    if (individualVals.length !== 3) {
        return false
    }

    const hour = parseInt(individualVals[0], 10)
    const minute = parseInt(individualVals[1], 10)
    const second = parseInt(individualVals[2], 10)

    if (Number.isNaN(hour) || Number.isNaN(minute) || Number.isNaN(second)) {
        return false
    }

    if (hour < 0 || minute < 0 || minute >= 60 || second < 0 || second >= 60) {
        return false
    }

    return true
}

export function startTimer(initialTimerValue = "00:00:00") {
    if (!timerValueIsValid(initialTimerValue)) {
        initialTimerValue = "00:00:00"
    } 

    //Get the initial number of milliseconds
    var timerValsArray = initialTimerValue.split(":")
    var hours = parseInt(timerValsArray[0], 10)
    var minutes = parseInt(timerValsArray[1], 10)
    var seconds = parseInt(timerValsArray[2], 10)

    seconds += minutes * 60
    seconds += hours * 3600

    timerRunning = true
    startTime = performance.now() - seconds * 1000;

    if (timerRunning) {
        requestAnimationFrame(incrementTimer)
    }
}

export function stopTimer() {
    timerRunning = false
}

export function incrementTimer() {
    const elapsedMs = performance.now() - startTime;
    const totalSeconds = Math.floor(elapsedMs / 1000);

    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    currentTimerString = `${String(hours).padStart(2,'0')}:${String(minutes).padStart(2,'0')}:${String(seconds).padStart(2,'0')}`;
    timerElem.innerHTML = currentTimerString;

    if (timerRunning) {
        requestAnimationFrame(incrementTimer)
    }
}

export function getCurrentTimerValue() {
    return timerValueIsValid(currentTimerString) ? currentTimerString : undefined
}