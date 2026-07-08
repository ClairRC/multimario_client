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
                <p style="display: inline; color: white; margin-left: 2%; font-size: 2em;">${category.getGameCount(raceCategory, numCollectibles)}</p>
                <p class="numeric-progress" style="color: white; font-size: 1em; text-align: right;">${toFrac(numCollectibles,category.getTotalCollectibles(raceCategory))}</p>
        </div>
    </div>
    `

    return cardHTML
}

//Fits text to not have to be truncated. Just do this once at setup.
export function fitText(element, maxFontSize = 2, minFontSize = 1.2) {
    var fontSize = maxFontSize
    element.style.fontSize = fontSize + "cqi"
    while(element.scrollWidth > element.clientWidth && fontSize >= minFontSize) {
        fontSize -= 0.1
        element.style.fontSize = fontSize + "cqi"
    }
}

//Turns a fraction value to be a diagnoal fraction
function toFrac(numerator, denomiator) {
    return `
    <em class="fraction"><span class="numerator">${numerator}</span>/<span class="denominator">${denomiator}</span></em>
    `
}