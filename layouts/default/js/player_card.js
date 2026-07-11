import * as category from "./category_info.js"

//Gets player card information
export function getPlayerCard(playerDisplayName, playerTwitchName, playerPos, pfpURL, numCollectibles, raceCategory) {
    //Get image and progress information based on the race category and num collectibles
    const bgImage = category.getCurrentBackgroundImage(raceCategory, numCollectibles)

    const cardHTML = `
    <div class="player-box" id="${playerTwitchName}" style="--bg-image: url('${bgImage}')">
        <div class="user-info">
            <img src="${pfpURL}" class="user-pfp">
            <p class="user-name">${playerDisplayName}</p>
            <p class="user-position">${playerPos}</p>
        </div>
        <div class="progress">
                <img class="icon" src="${category.getCurrentIconImage(raceCategory, numCollectibles)}">
                <p class="game-progress" style="display: inline; color: white; margin-left: 2%; font-size: 2em;">${category.getGameCount(raceCategory, numCollectibles)}</p>
                <p class="numeric-progress" style="color: white; font-size: 1em; text-align: right;">${toFrac(Math.min(numCollectibles, category.getTotalCollectibles(raceCategory)),category.getTotalCollectibles(raceCategory))}</p>
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

export function updateCardImages(playerTwitchName, numCollected, raceCategory) {
    var card = document.getElementById(playerTwitchName)
    if (card.className == "placeholder") {
        return
    }
    card.style.setProperty('--bg-image', `url('${category.getCurrentBackgroundImage(raceCategory, numCollected)}')`);
    card.querySelector(".icon").src = category.getCurrentIconImage(raceCategory, numCollected)
}

export function updatePlayerProgress(playerTwitchName, numCollected, raceCategory) {
    var card = document.getElementById(playerTwitchName)
    if (card.className == "placeholder") {
        return
    }
    card.querySelector(".game-progress").innerHTML = category.getGameCount(raceCategory, numCollected)
    card.querySelector(".numeric-progress").innerHTML = toFrac(Math.min(numCollected, category.getTotalCollectibles(raceCategory)),category.getTotalCollectibles(raceCategory))
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