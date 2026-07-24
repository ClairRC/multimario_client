//Includes functionality for logging to the logbox on the main page

function logMessage(msg) {
    var now = new Date()
    var time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`

    logInfoBox.value += `[${time}] ${msg}\n`
    logInfoBox.scrollTop = logInfoBox.scrollHeight
}

function logError(msg) {
    var now = new Date()
    var time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`

    logInfoBox.value += `[${time}] ERROR: ${msg}\n`
    logInfoBox.scrollTop = logInfoBox.scrollHeight
}

function logWarning(msg) {
    var now = new Date()
    var time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`

    logInfoBox.value += `[${time}] WARNING: ${msg}\n`
    logInfoBox.scrollTop = logInfoBox.scrollHeight
}