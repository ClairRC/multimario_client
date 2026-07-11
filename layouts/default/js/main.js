import { onInit, onUpdate } from '../api.js'
import * as player from './player_card.js'
import * as category from './category_info.js'
import * as timer from './timer.js'

//This code is high key a mess godspeed

/*TODO:
    - add a quit/finished/etc status
    - move quitters to the end and finishes before them
    - make better finisher picture
*/

//Global variables for state
var currentPlayerPlacements = []
var currentRaceCategory = ""
var currentCardAnimations = {} //Stores card animations globally so they can be cancelled if they overlap

//Cache to know what text needs to be updated since that's a bottleneck
var updateText = {}

//Durations (ms) for how long to STAY on each page before turning, indexed by page number.
//Page 0 (first, top-ranked) stays up longest; falls back to lastDuration for any page beyond this list.
const pageDurations = [10000, 10000, 10000] // 60s, 30s, 15s
const fallbackPageDuration = 10000 // any page beyond the list above uses this
var pageInterval = null
var pageNum = 0


const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

onInit(async (data) => {
    if (pageInterval !== null) {
        clearTimeout(pageInterval)
        pageInterval = null
    }
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
        var aFinished = a.num_collected >= category.getTotalCollectibles(data.race_category)
        var bFinished = b.num_collected >= category.getTotalCollectibles(data.race_category)

        //Both finished. rank by finish time, faster time first
        if (aFinished && bFinished) {
            return timeToSeconds(a.time) - timeToSeconds(b.time)
        }

        //Finished player goes after the active one
        if (aFinished) return 1
        if (bFinished) return -1

        //Neither finished. rank by progress, most collected first
        return b.num_collected - a.num_collected || timeToSeconds(b.Estimate) - timeToSeconds(a.Estimate) || a.twitch_name.localeCompare(b.twitch_name)
    })

    //Sort records to get current placements
    var sortedRecords = playerRecords
        .slice()
    sortedRecords.sort((a, b) => {
        return b.num_collected - a.num_collected || timeToSeconds(a.time) - timeToSeconds(b.time)|| a.twitch_name.localeCompare(b.twitch_name)
    })
    var placementMap = {}
    sortedRecords.forEach((r, idx) => { placementMap[r.twitch_name] = idx + 1 })

    playerRecords.forEach((record, i) => {
        var card = player.getPlayerCard(record.player_name, record.twitch_name, placementMap[record.twitch_name], record.pfp_url, record.num_collected, data.race_category)
        var isInTop3 = i <= 2
        
        if (isInTop3) {
            top3.innerHTML += card
        } else {
            statsGrid.innerHTML += card
        }

        //Update player cache
        currentPlayerPlacements.push(record)
    });

    //Add placeholder cards to the DOM just for page switching
    for(let i = 0; i < 25; i++) {
        statsGrid.innerHTML += player.getPlaceHolderCard(`__placeholder${i}`)
    }

    //For placeholders that are not on screen, set their display to none so they don't get in the way
    for (let i = 0; i < ((currentPlayerPlacements.length-3) % 25); i++) {
        var card = document.getElementById(`__placeholder${i}`)
        card.style.display = "none"
    }

    while((currentPlayerPlacements.length-3) % 25 !== 0) {
        currentPlayerPlacements.push({"twitch_name": `__placeholder${(currentPlayerPlacements.length-3) % 25}`, isPlaceHolder: true})
    }

    //Reset timer and set current timer value
    timer.stopTimer()
    if (data.timer_running) {
        timer.startTimer(data.timer_value)
    }

    document.fonts.ready.then(() => {
        fixAllTextSizing()
    });

    schedulePageTurn()

    {
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
         testDatas.push({
            "twitch_name": "nathancarter602",
            "num_collected": 306,
        })
        testDatas.push({
            "twitch_name": "odme_",
            "num_collected": 270,
        })
        testDatas.push({
            "twitch_name": "zgamut",
            "num_collected": 150,
        })
        testDatas.push({
            "twitch_name": "galax_v",
            "num_collected": 210,
        })

        await sleep(5000)
        for (let i = 0; i < testDatas.length; i++) {
            updatePlayerCount(testDatas[i])
            await sleep(300 * (i+1))
        }

        //Change nathan carter name
        var testNameChange = {
            "twitch_name": "nathancarter602",
            "player_name": "silly guy",
        }
        updatePlayerName(testNameChange)
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
        timer.timerUpdate(data)
    }
})*/

function updatePlayerCount(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.num_collected = data.num_collected
            updateText[record.twitch_name] = true
        }
    });
    updateCardPlacements(500)
}

function updatePlayerName(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.player_name = data.player_name
            updateText[record.twitch_name] = true
            player.updatePlayerName(record.player_name, record.twitch_name)
            updatePlayerCards()
        }
    });
}

//Updates player cards. This is pretty slow because it just completely redoes the whole thing.
function updateCardPlacements(animationLength) {
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

        if (record.isPlaceHolder) {
            return
        }

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
            duration: animationLength,
            easing: 'ease-out',
            fill: 'both'
        });

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

    sortPlayers()

    var playerRecords = currentPlayerPlacements

    var realRecordsSorted = playerRecords
        .filter(r => !r.isPlaceHolder)
        .slice()
    realRecordsSorted.sort((a, b) => {
        return b.num_collected - a.num_collected || timeToSeconds(a.time) - timeToSeconds(b.time)|| a.twitch_name.localeCompare(b.twitch_name)
    })

    var placementMap = {}
    realRecordsSorted.forEach((r, idx) => { placementMap[r.twitch_name] = idx + 1 })

    //Get each player's card and sauce them on the screen
    playerRecords.forEach((record, i) => {
        var card = document.getElementById(record.twitch_name)

        if (updateText[record.twitch_name]) {
            fixCardTextSizing(card)
            updateText[record.twitch_name] = false
        }

        card.style.order = i

        var playerPlacementNum = i < 3 ? (i+1) : (i + (25 * pageNum)) + 1

        var targetContainer = i < 3 ? top3 : statsGrid

        if (card.parentElement !== targetContainer) {
            targetContainer.appendChild(card);
        }

        card.style.order = i

        if (record.isPlaceHolder) {
            return
        }

        player.updatePlayerPlacement(record.twitch_name, placementMap[record.twitch_name])
        player.updateCardImages(record.twitch_name, record.num_collected, currentRaceCategory)
        player.updatePlayerProgress(record.twitch_name, record.num_collected, currentRaceCategory)
    });
}

function getPageDuration(pageIndex) {
    if (pageIndex < pageDurations.length) {
        return pageDurations[pageIndex]
    }
    return fallbackPageDuration
}

function schedulePageTurn() {
    if (pageInterval !== null) {
        clearTimeout(pageInterval)
        pageInterval = null
    }

    var numPlayers = currentPlayerPlacements.length
    var numPages = Math.ceil(((numPlayers-3)/25.0))
    if (numPages <= 1) return //nothing to page through

    var delay = getPageDuration(pageNum)

    pageInterval = setTimeout(async () => {
        await updatePage(400)
        schedulePageTurn() //reschedule for the NEW pageNum, after the turn completes
    }, delay)
}

async function updatePage(transitionLength) {
    var numPlayers = currentPlayerPlacements.length
    var numPages = Math.ceil(((numPlayers-3)/25.0))
    if (numPages <= 1) return

    var statsGrid = document.querySelector(".stats-grid")

    //distance from the top of the current page's first card
    var pageHeight = getPageHeight(3, 25)
    if (!pageHeight) return

    //scroll the whole grid up smoothly
    statsGrid.style.transition = `transform ${transitionLength}ms ease-in-out`
    statsGrid.style.transform = `translateY(-${pageHeight}px)`

    await new Promise(resolve => {
        statsGrid.addEventListener('transitionend', resolve, { once: true })
    })

    pageNum = (pageNum + 1) % numPages
    updatePlayerCards()

    statsGrid.style.transition = 'none'
    statsGrid.style.transform = 'translateY(0px)'
    void statsGrid.offsetHeight // force reflow before re-enabling transitions
    statsGrid.style.transition = ''
}

function getPageHeight(startIndex, pageSize) {
    var current = currentPlayerPlacements[startIndex]
    var next = currentPlayerPlacements[startIndex + pageSize]
    if (!current || !next) return 0

    var elCurrent = document.getElementById(current.twitch_name)
    var elNext = document.getElementById(next.twitch_name)
    if (!elCurrent || !elNext) return 0

    return elNext.getBoundingClientRect().top - elCurrent.getBoundingClientRect().top
}

function sortPlayers() {
    //Sort real records so it goes up in the right order
    var fakeRecords = currentPlayerPlacements.filter(r => r.isPlaceHolder)
    var realRecords = currentPlayerPlacements.filter(r => !r.isPlaceHolder)
    realRecords.sort((a, b) => {
        var aFinished = a.num_collected >= category.getTotalCollectibles(currentRaceCategory)
        var bFinished = b.num_collected >= category.getTotalCollectibles(currentRaceCategory)

        //Both finished. rank by finish time, faster time first
        if (aFinished && bFinished) {
            return timeToSeconds(a.time) - timeToSeconds(b.time)
        }

        //Finished player goes after the active one
        if (aFinished) return 1
        if (bFinished) return -1

        //Neither finished. rank by progress, most collected first
        return b.num_collected - a.num_collected || timeToSeconds(b.Estimate) - timeToSeconds(a.Estimate) || a.twitch_name.localeCompare(b.twitch_name)
    })

    //Add the fake records back in
    for (let i = 0; i < fakeRecords.length; i++) {
        realRecords.push(fakeRecords[i])
    }

    //After sorted by placement, sort by page number
    var newPlacements = []
    var numPlayers = realRecords.length

    //Add top 3 because they are always on the top
    for (let i = 0; i < 3; i++) {
        newPlacements.push(realRecords[i])
    }

    //For the rest, go through each page starting at the current page
    var numPages = (numPlayers-3)/25
    for (let i = pageNum; i < numPages + pageNum; i++) {
        var currentPageNum = i % numPages
        var jBound = ((3 + (currentPageNum * 25)) + 25)
        for(let j = (3 + (currentPageNum * 25)); j < jBound; j++) {
            newPlacements.push(realRecords[j])
        }
    }
    currentPlayerPlacements = [...newPlacements]
}

function fixCardTextSizing(cardElement) {
    //Get user name elements to fit them properly
    var userNames = cardElement.querySelectorAll(".user-name")
    userNames.forEach(element => {
        player.fitText(element, 1.9, 1.4)
    });

    var progressValues = cardElement.querySelectorAll(".game-progress")
    progressValues.forEach(element => {
        player.fitText(element, 3.3 , 1.5)
    });
}

function fixAllTextSizing() {
    //Get user name elements to fit them properly
    var userNames = document.querySelectorAll(".user-name")
    userNames.forEach(element => {
        player.fitText(element, 1.9, 1.4)
    });

    var progressValues = document.querySelectorAll(".game-progress")
    progressValues.forEach(element => {
        player.fitText(element, 3.3 , 1.5)
    });

    //Total progress needs to have a smaller max size
    progressValues = document.querySelectorAll(".numeric-progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.2, 0.0)
    });
}

function timeToSeconds(time) {
    if (time === undefined) {
        return Number.MAX_SAFE_INTEGER
    }

    var hoursString = time.split(":")[0]
    var minutesString = time.split(":")[1]
    var secondsString = time.split(":")[2]

    var out = parseInt(hoursString) * 3600
    out += parseInt(minutesString) * 60
    out += parseInt(secondsString)

    return out
}