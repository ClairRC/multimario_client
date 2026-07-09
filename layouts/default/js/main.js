import { onInit, onUpdate } from '../api.js'
import * as player from './player_card.js'
import * as category from './category_info.js'

//This code is high key a mess godspeed

//Global variables for state
var currentPlayerPlacements = []
var currentRaceCategory = ""
var currentCardAnimations = {} //Stores card animations globally so they can be cancelled if they overlap

var timerElem = document.querySelector(".timer")
var timerRunning = false
var startTime = undefined

var pageNum = 0

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

onInit(async (data) => {
    currentPlayerPlacements = []
    currentRaceCategory = data.race_category
    currentCardAnimations = {}

    var statsGrid = document.querySelector(".stats-grid")
    var top3 = document.querySelector(".top-3")

    //Clear out the player cards since this is init
    statsGrid.innerHTML = ""
    top3.innerHTML = ""

    var playerRecords = data.records

    //Sort player records so it goes up in the right order
    playerRecords.sort((a, b) => {
        return b.time-a.time || b.num_collected - a.num_collected
    })

    //Get each player's card and sauce them on the screen
    playerRecords.forEach((record, i) => {
        const pictureURL = "https://static-cdn.jtvnw.net/jtv_user_pictures/6dece3b4-ebec-47a8-b735-771a62e825ec-profile_image-70x70.png"
        var card = player.getPlayerCard(record.player_name, record.twitch_name, i+1, pictureURL, record.num_collected, data.race_category)
        var isInTop3 = i <= 2
        
        if (isInTop3) {
            top3.innerHTML += card
        } else {
            statsGrid.innerHTML += card
        }

        //Update player cache
        currentPlayerPlacements.push(record)
    });

    //Reset timer and set current timer value
    stopTimer()
    if (data.timer_running) {
        startTimer(data.timer_value)
    }

    document.fonts.ready.then(() => {
        fixAllTextSizing()
    });

    var testDatas = []
    testDatas.push({
        "twitch_name": "zgamut",
        "num_collected": 100,
    })        
    testDatas.push({
        "twitch_name": "bird650",
        "num_collected": 150,
    }) 
    testDatas.push({
        "twitch_name": "galax_v",
        "num_collected": 210,
    }) 
    testDatas.push({
        "twitch_name": "jukatox",
        "num_collected": 280,
    })
    testDatas.push({
        "twitch_name": "odme_",
        "num_collected": 250,
    })
    testDatas.push({
        "twitch_name": "muimania",
        "num_collected": 80,
    }) 
    testDatas.push({
        "twitch_name": "odme_",
        "num_collected": 365,
    })
    testDatas.push({
        "twitch_name": "gemoflol",
        "num_collected": 95,
    })
    testDatas.push({
        "twitch_name": "nathancarter602",
        "num_collected": 289,
    })
    testDatas.push({
        "twitch_name": "galax_v",
        "num_collected": 700,
    })

    await sleep(2000)
    for (let i = 0; i < testDatas.length; i++) {
        updatePlayerCount(testDatas[i])
        await sleep(500 * (i+1))
    }
})


/*
onUpdate((data) => {
    if (data.kind === "player_count") {
        updatePlayerCount(data)
    }

    if (data.kind === "player_name") {
        updatePlayerName(data)
    }

    if (data.kind === "timer") {
        timerUpdate(data)
    }
})*/

async function updatePlayerCount(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.num_collected = data.num_collected
        }
    });
    updateCardPlacements()
}

function updatePlayerName(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.player_name = data.player_name
            player.updatePlayerName(record.player_name, record.twitch_name)
        }
    });
}

//Updates player cards. This is pretty slow because it just completely redoes the whole thing.
function updateCardPlacements() {
    //Cache of current card locations before animating
    var initialLocationMap = {}

    //Get the current location for each card
    currentPlayerPlacements.forEach(record  => {
        //Get card element
        var card = document.getElementById(record.twitch_name)

        //Add the location to the initial location map
        initialLocationMap[record.twitch_name] = card.getBoundingClientRect()
    });

    //Update the locations of the cards
    updatePlayerCards()

    //Go through each player placement and animate them to their initial location
    currentPlayerPlacements.forEach(record => {
        var card = document.getElementById(record.twitch_name)

        //If this card already has an animation, cancel it
        if (currentCardAnimations[record.twitch_name]) {
            currentCardAnimations[record.twitch_name].cancel()
        }

        //Get locations
        var first = initialLocationMap[record.twitch_name]
        var last = card.getBoundingClientRect()

        const deltaX = first.left - last.left;
        const deltaY = first.top - last.top;
        const deltaW = first.width / last.width;
        const deltaH = first.height / last.height;

        const anim = card.animate([{
            transformOrigin: 'top left',
            transform: `
            translate(${deltaX}px, ${deltaY}px)
            scale(${deltaW}, ${deltaH})
            `
            }, {
            transformOrigin: 'top left',
            transform: 'none'
            }], {
            duration: 500,
            easing: 'ease-out',
            fill: 'both'
        });

        fixCardTextSizing(card)

        //Add this to the current animations and when its finished delete it
        currentCardAnimations[record.twitch_name] = anim
        anim.onfinish = () => {
            if (currentCardAnimations[record.twitch_name] === anim) {
                delete currentCardAnimations[record.twitch_name]
            }
        }
    });
}

function updatePlayerCards() {
    var statsGrid = document.querySelector(".stats-grid")
    var top3 = document.querySelector(".top-3")

    var playerRecords = currentPlayerPlacements

    //Sort player records so it goes up in the right order
    playerRecords.sort((a, b) => {
        return b.time-a.time || b.num_collected - a.num_collected
    })

    //Get each player's card and sauce them on the screen
    playerRecords.forEach((record, i) => {
        var card = document.getElementById(record.twitch_name)
        card.style.order = i
        var targetContainer = i < 3 ? top3 : statsGrid

        if (card.parentElement !== targetContainer) {
            targetContainer.appendChild(card);
        }

        card.style.order = i

        //Update card visuals
        player.updateCardImages(record.twitch_name, record.num_collected, currentRaceCategory)
        player.updatePlayerPlacement(record.twitch_name, (i+1))
        player.updatePlayerProgress(record.twitch_name, record.num_collected, currentRaceCategory)
    });

    fixAllTextSizing()
}

function fixCardTextSizing(cardElement) {
    //Get user name elements to fit them properly
    var userNames = cardElement.querySelectorAll(".user-name")
    userNames.forEach(element => {
        player.fitText(element, 1.9, 1.0)
    });

    var progressValues = cardElement.querySelectorAll(".progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.8, 1.5)
    });

    //Total progress needs to have a smaller max size
    progressValues = cardElement.querySelectorAll(".numeric-progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.2, 0.0)
    });
}

function fixAllTextSizing() {
    //Get user name elements to fit them properly
    var userNames = document.querySelectorAll(".user-name")
    userNames.forEach(element => {
        player.fitText(element, 1.9, 1.0)
    });

    var progressValues = document.querySelectorAll(".progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.8 , 1.5)
    });

    //Total progress needs to have a smaller max size
    progressValues = document.querySelectorAll(".numeric-progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.2, 0.0)
    });
}

function timerUpdate(data) {
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
function timerValueIsValid(timerValue) {
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

function startTimer(initialTimerValue = "00:00:00") {
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

function stopTimer() {
    timerRunning = false
}

function incrementTimer() {
    var timer = timerElem

    const elapsedMs = performance.now() - startTime;
    const totalSeconds = Math.floor(elapsedMs / 1000);

    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    timer.innerHTML = `${String(hours).padStart(2,'0')}:${String(minutes).padStart(2,'0')}:${String(seconds).padStart(2,'0')}`;

    if (timerRunning) {
        requestAnimationFrame(incrementTimer)
    }
}