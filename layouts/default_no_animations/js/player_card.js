import * as category from "./category_info.js"

//Gets player card information
export function getPlayerCard(playerDisplayName, playerTwitchName, playerPos, pfpURL, numCollectibles, playerStatus, playerTime, raceCategory) {
    if (playerStatus !== "running") {
        //Get image and progress information based on the race category and num collectibles
        const bgImage = category.getDNFBackgroundImage(raceCategory)

        const cardHTML = `
        <div class="player-box" id="${playerTwitchName}" style="--bg-image: url('${bgImage}')">
            <div class="user-info">
                <img src="${pfpURL}" class="user-pfp">
                <p class="user-name">${playerDisplayName}</p>
                <p class="user-position">${playerPos}</p>
            </div>
            <div class="progress">
                    <img class="icon" src="${category.getCurrentIconImage(raceCategory, numCollectibles)}" style="display: none;">
                    <p class="quit-text" style="display: inline; margin-left: 4%; font-size: 2em;">${playerStatus}</p>
                    <p class="game-progress" style="display: none; color: white; margin-left: 2%; font-size: 2em;"></p>
                    <p class="numeric-progress" style="color: white; font-size: 1em; text-align: right;">${toFrac(Math.min(numCollectibles, category.getTotalCollectibles(raceCategory)),category.getTotalCollectibles(raceCategory))}</p>
            </div>
        </div>
        `

        return cardHTML
    }

    //Get image and progress information based on the race category and num collectibles
    const bgImage = category.getCurrentBackgroundImage(raceCategory, numCollectibles)

    var displayFinishTime = numCollectibles >= category.getTotalCollectibles(raceCategory) && isValidFinishTime(playerTime)
    var playerIsFinished = numCollectibles >= category.getTotalCollectibles(raceCategory)

    var iconDisplay = displayFinishTime ? "none" : "block"
    var progressText = displayFinishTime ? playerTime : category.getGameCount(raceCategory, numCollectibles)
    var fractionDisplay = displayFinishTime ? "none": "block"
    var progressJustifyContent = displayFinishTime ? "center" : "flex-start"
    var progressTextAlign = displayFinishTime ? "center" : "right"
    var strokeColor = displayFinishTime ? "goldenrod" : "black"
    var progressTextColor = displayFinishTime ? "rgb(255, 230, 167)" : "white"
    var positionColor = displayFinishTime || playerIsFinished ? "gold" : "white"
    var positionStroke = displayFinishTime || playerIsFinished ? "goldenrod" : "black"

    const cardHTML = `
    <div class="player-box" id="${playerTwitchName}" style="--bg-image: url('${bgImage}')">
        <div class="user-info">
            <img src="${pfpURL}" class="user-pfp">
            <p class="user-name">${playerDisplayName}</p>
            <p class="user-position" style="color: ${positionColor}; --stroke-color: ${positionStroke};" >${playerPos}</p>
        </div>
        <div class="progress" style="justify-content: ${progressJustifyContent}">
                <img class="icon" src="${category.getCurrentIconImage(raceCategory, numCollectibles)}" style="display: ${iconDisplay};">
                <p class="game-progress" style="display: inline; color: ${progressTextColor}; margin-left: 2%; font-size: 2em; --stroke-color: ${strokeColor};">${progressText}</p>
                <p class="quit-text" style="display: none; margin-left: 4%; font-size: 2em;"></p>
                <p class="numeric-progress" style="color: white; font-size: 1em; text-align: ${progressTextAlign}; display: ${fractionDisplay};">${toFrac(Math.min(numCollectibles, category.getTotalCollectibles(raceCategory)),category.getTotalCollectibles(raceCategory))}</p>
        </div>
    </div>
    `

    return cardHTML
}

export function updatePlayerName(playerNewDisplay, playerTwitchName) {
    var card = document.getElementById(playerTwitchName)
    if (card.className == "placeholder") {
        return
    }
    card.querySelector(".user-name").innerHTML = playerNewDisplay
}

export function updatePlayerPlacement(playerTwitchName, newPlacement) {
    var card = document.getElementById(playerTwitchName)
    if (card.className == "placeholder") {
        return
    }
    card.querySelector(".user-position").innerHTML = newPlacement
}

export function updateCardImages(playerTwitchName, numCollected, raceCategory, playerStatus) {
    var card = document.getElementById(playerTwitchName)
    if (card.className == "placeholder") {
        return
    }

    var cardIcon = card.querySelector(".icon")

    if (playerStatus !== "running") {
        card.style.setProperty('--bg-image', `url('${category.getDNFBackgroundImage(raceCategory)}')`);
        cardIcon.style.display = "none"
        
        return
    }

    card.style.setProperty('--bg-image', `url('${category.getCurrentBackgroundImage(raceCategory, numCollected)}')`);
    cardIcon.style.display = "block"
    cardIcon.src = category.getCurrentIconImage(raceCategory, numCollected)
}

export function updatePlayerProgress(playerTwitchName, numCollected, raceCategory, playerStatus, playerTime) {
    var card = document.getElementById(playerTwitchName)
    if (card.className == "placeholder") {
        return
    }

    var gameProgress = card.querySelector(".game-progress")
    var quitText = card.querySelector(".quit-text")

    if (playerStatus !== "running") {
        quitText.innerHTML = playerStatus
        quitText.style.display = "inline"
            
        gameProgress.style.display = "none"

        return
    }

    quitText.style.display = "none"
    gameProgress.style.display = "inline"

    card.querySelector(".numeric-progress").innerHTML = toFrac(Math.min(numCollected, category.getTotalCollectibles(raceCategory)),category.getTotalCollectibles(raceCategory))

    var displayFinishTime = numCollected >= category.getTotalCollectibles(raceCategory) && isValidFinishTime(playerTime)
    var playerIsFinished = numCollected >= category.getTotalCollectibles(raceCategory)

    var iconDisplay = displayFinishTime ? "none" : "block"
    var progressText = displayFinishTime ? playerTime : category.getGameCount(raceCategory, numCollected)
    var fractionDisplay = displayFinishTime ? "none": "block"
    var progressJustifyContent = displayFinishTime ? "center" : "flex-start"
    var progressTextAlign = displayFinishTime ? "center" : "right"
    var strokeColor = displayFinishTime ? "goldenrod" : "black"
    var progressTextColor = displayFinishTime ? "rgb(255, 230, 167)" : "white"
    var positionColor = displayFinishTime || playerIsFinished ? "gold" : "white"
    var positionStroke = displayFinishTime || playerIsFinished ? "goldenrod" : "black"

    card.querySelector(".icon").style.display = iconDisplay
    card.querySelector(".game-progress").innerHTML = progressText
    card.querySelector(".numeric-progress").style.display = fractionDisplay
    card.querySelector(".progress").style.justifyContent = progressJustifyContent
    card.querySelector(".game-progress").style.textAlign = progressTextAlign
    card.querySelector(".game-progress").style.color = progressTextColor
    card.querySelector(".game-progress").style.setProperty('--stroke-color', strokeColor)
    card.querySelector(".user-position").style.setProperty('--stroke-color', strokeColor)
    card.querySelector(".user-position").style.color = positionColor
}

//Fits text to not have to be truncated. Just do this once at setup.
export function fitText(element, maxFontSize = 2, minFontSize = 1.2) {
    var fontSize = maxFontSize
    element.style.fontSize = fontSize + "cqi"
    while(element.scrollWidth > element.clientWidth && fontSize >= minFontSize) {
        fontSize -= 0.2
        element.style.fontSize = fontSize + "cqi"
    }
}

export function getPlaceHolderCard(id) {
    return `<div class="placeholder" id="${id}"></div>`
}

//Turns a fraction value to be a diagnoal fraction
function toFrac(numerator, denomiator) {
    return `
    <em class="fraction"><span class="numerator">${numerator}</span><span style="-webkit-text-stroke: 0.0cqi; "> /</span><span class="denominator">${denomiator}</span></em>
    `
}

function isValidFinishTime(time) {
    if (time === undefined || time === null) {
        return false
    }

    var timeParts = time.split(":")

    if (timeParts.length !== 3) {
        return false
    }

    var hour = parseInt(timeParts[0], 10)
    var minute = parseInt(timeParts[1], 10)
    var second = parseInt(timeParts[2], 10)

    if (Number.isNaN(hour) || Number.isNaN(minute) || Number.isNaN(second)) {
        return false
    }

    if (hour > 99) { return false }
    if (minute > 59) {return false}
    if (second > 59) {return false}

    return true
}