import { onInit, onUpdate } from '../api.js'
import * as player from './player_card.js'
import * as category from './category_info.js'
import * as timer from './timer.js'

//This code is high key a mess godspeed

/*TODO:
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
const pageDurations = [30000, 20000, 15000] // 30s, 20s, 15s
const fallbackPageDuration = 10000 // any page beyond the list above uses this
var pageInterval = null
var pageNum = 0

const cardsPerPage = 25
const nextPageEnd = 3 + cardsPerPage * 2

var pageGeneration = 0 //So that functions running asyncronously can tell when onInit has been called again so they don't run

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

onInit((data) => {
    pageGeneration++
    if (pageInterval !== null) {
        clearTimeout(pageInterval)
        pageInterval = null
    }
    currentPlayerPlacements = []
    currentRaceCategory = data.race_category
    currentCardAnimations = {}
    updateText = {}
    pageNum = 0

    var gridContainer = document.querySelector(".grid-scroller")
    var statsGrid = document.querySelector(".stats-grid")
    var statsGridBottom = document.querySelector(".stats-grid-bottom")
    var top3 = document.querySelector(".top-3")

    //Clear out the player cards since this is init
    statsGrid.innerHTML = ""
    top3.innerHTML = ""
    statsGridBottom.innerHTML = ""

    statsGrid.style.transition = 'none'
    statsGrid.style.transform = 'translateY(0px)'
    statsGridBottom.style.transition = 'none'
    statsGridBottom.style.transform = 'translateY(0px)'

    var playerRecords = data.records

    //First, sort by rank to get placement
    var placementMap = getPlacementMap(playerRecords)
    
    //Next, sort by display order
    playerRecords.sort(orderDisplayComparator)

    playerRecords.forEach((record, i) => {
        //Make sure that if the twitch name hasn't been changed, we apply proper capitalization
        if (record.player_name === record.twitch_name) {
            record.player_name = record.twitch_display_name
        }

        var card = player.getPlayerCard(record.player_name, record.twitch_name, placementMap[record.twitch_name], record.pfp_url, record.num_collected, record.status, record.time, data.race_category)
        var isInTop3 = i <= 2
        var isNormal = i < 28
        
        if (isInTop3) {
            top3.innerHTML += card
        } else if (isNormal) {
            statsGrid.innerHTML += card
        } else {
            statsGridBottom.innerHTML += card
        }

        //Update player cache
        currentPlayerPlacements.push(record)
    });

    var remainder = (currentPlayerPlacements.length - 3) % 25
    var paddingNeeded = remainder === 0 ? 0 : 25 - remainder

    //Add placeholder cards to the DOM just for page switching
    for(let i = 0; i < 25; i++) {
        statsGridBottom.innerHTML += player.getPlaceHolderCard(`__placeholder${i}`)
    }

    //Fix text sizing and fill in the rest of the player cards once we're ready for that
    document.fonts.ready.then(() => {
        fixAllTextSizing()

        //Add exactly the placeholders needed to round out the last page
        for (let i = 0; i < paddingNeeded; i++) {
            currentPlayerPlacements.push({"twitch_name": `__placeholder${i}`, isPlaceHolder: true})
        }

        //Hide whichever placeholder cards weren't used
        for (let i = paddingNeeded; i < 25; i++) {
            var card = document.getElementById(`__placeholder${i}`)
            card.style.display = "none"
        }

        playerRecords.forEach((record, i) => {
        if (i >= nextPageEnd) {
            var card = document.getElementById(record.twitch_name)
            if (card) card.style.display = "none"
        }
        })
    });

    //Reset timer and set current timer value
    timer.timerUpdate(data)

    schedulePageTurn()
})


onUpdate((data) => {
    if (data.kind === "player_count") {
        updatePlayerCount(data)
    }

    if (data.kind === "player_name") {
        updatePlayerName(data)
    }

    if (data.kind === "player_status") {
        updatePlayerStatus(data)
    }

    if (data.kind === "player_time") {
        updatePlayerTime(data)
    }

    if (data.kind === "timer") {
        timer.timerUpdate(data)
    }
})

function updatePlayerCount(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.num_collected = data.num_collected
            //If this player has just finished, set their finish time to be the current timer value
            if (record.num_collected >= category.getTotalCollectibles(currentRaceCategory)) {
                record.time = timer.getCurrentTimerValue()
            }

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

function updatePlayerStatus(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.status = data.status
            updateText[record.twitch_name] = true
        }
    });
    updateCardPlacements(500)
}


function updatePlayerTime(data) {
    //Update this player's record in the cache
    currentPlayerPlacements.forEach(record => {
        if (record.twitch_name === data.twitch_name) {
            record.time = data.time
            updateText[record.twitch_name] = true
        }
    });
    updateCardPlacements(500)
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
    var bottom5 = document.querySelector(".stats-grid-bottom")

    sortPlayers()

    var playerRecords = currentPlayerPlacements
    var realRecordsSorted = playerRecords.filter(r => !r.isPlaceHolder).slice()
    realRecordsSorted.sort(orderRankComparator)
    var placementMap = getPlacementMap(realRecordsSorted)

    playerRecords.forEach((record, i) => {
        var card = document.getElementById(record.twitch_name)
        card.style.order = i

        var targetContainer
        if (i < 3) {
            targetContainer = top3
        } else if (i < 28) {
            targetContainer = statsGrid
        } else {
            targetContainer = bottom5
        }

        if (card.parentElement !== targetContainer) {
            targetContainer.appendChild(card);
        }

        // Only the current page + the immediate next page should take up space.
        // Anything further out is just waiting its turn — keep it out of the layout.
        card.style.display = (i < nextPageEnd) ? "" : "none"

        if (record.isPlaceHolder) {
            return
        }

        player.updatePlayerPlacement(record.twitch_name, placementMap[record.twitch_name])
        player.updateCardImages(record.twitch_name, record.num_collected, currentRaceCategory, record.status)
        player.updatePlayerProgress(record.twitch_name, record.num_collected, currentRaceCategory, record.status, record.time)

        if (updateText[record.twitch_name]) {
            fixCardTextSizing(card)
            updateText[record.twitch_name] = false
        }
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
    var currentPageGeneration = pageGeneration

    var numPlayers = currentPlayerPlacements.length
    var numPages = Math.ceil(((numPlayers-3)/25.0))
    if (numPages <= 1) return

    var statsGrid = document.querySelector(".stats-grid")
    var statsGridBottom = document.querySelector(".stats-grid-bottom")

    //distance from the top of the current page's first card
    var pageHeight = getPageHeight(3, 25)
    if (!pageHeight) return

    //scroll the whole grid up smoothly
    statsGrid.style.transition = `transform ${transitionLength}ms ease-in-out`
    statsGrid.style.transform = `translateY(-${pageHeight}px)`
    statsGridBottom.style.transition = `transform ${transitionLength}ms ease-in-out`
    statsGridBottom.style.transform = `translateY(-${pageHeight}px)`

    await new Promise(resolve => {
        statsGrid.addEventListener('transitionend', resolve, { once: true })
        statsGridBottom.addEventListener('transitionend', resolve, { once: true })
    })

    //A new page was generated, don't do the animation.
    if (currentPageGeneration !== pageGeneration) {
        return
    }

    pageNum = (pageNum + 1) % numPages
    updatePlayerCards()

    statsGrid.style.transition = 'none'
    statsGrid.style.transform = 'translateY(0px)'
    void statsGrid.offsetHeight // force reflow before re-enabling transitions
    statsGrid.style.transition = ''

    statsGridBottom.style.transition = 'none'
    statsGridBottom.style.transform = 'translateY(0px)'
    void statsGridBottom.offsetHeight // force reflow before re-enabling transitions
    statsGridBottom.style.transition = ''
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
    realRecords.sort(orderDisplayComparator)

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
        player.fitText(element, 2.2, 1.3)
    });

    var progressValues = cardElement.querySelectorAll(".game-progress")
    progressValues.forEach(element => {
        if (element.style.display !== "none") {
            player.fitText(element, 3.3 , 1.5)
        }
    });

    var quitText = cardElement.querySelectorAll(".quit-text")
    quitText.forEach(element => {
        if (element.style.display !== "none") {
            player.fitText(element, 3.0, 0.0)
        }
    });

    var userPos = cardElement.querySelectorAll(".user-position")
    userPos.forEach(element => {
        if (element.style.display !== "none") {
            player.fitText(element, 2.8, 1.5)
        }
    });
}

function fixAllTextSizing() {
    //Get user name elements to fit them properly
    var userNames = document.querySelectorAll(".user-name")
    userNames.forEach(element => {
        player.fitText(element, 2.2, 1.3)
    });

    var progressValues = document.querySelectorAll(".game-progress")
    progressValues.forEach(element => {
        if (element.style.display !== "none") {
            player.fitText(element, 3.3 , 1.5)
        }
    });

    var quitText = document.querySelectorAll(".quit-text")
    quitText.forEach(element => {
        if (element.style.display !== "none") {
            player.fitText(element, 3.0, 0.0)
        }
    });

    var userPos = document.querySelectorAll(".user-position")
    userPos.forEach(element => {
        if (element.style.display !== "none") {
            player.fitText(element, 2.8, 1.5)
        }
    });

    //Total progress needs to have a smaller max size
    progressValues = document.querySelectorAll(".numeric-progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.2, 0.0)
    });
}

function timeToSeconds(time) {
    if (!time || typeof time !== "string") {
        return Number.MAX_SAFE_INTEGER
    }

    var parts = time.split(":")
    if (parts.length !== 3) {
        return Number.MAX_SAFE_INTEGER
    }

    var out = parseInt(parts[0]) * 3600 + parseInt(parts[1]) * 60 + parseInt(parts[2])

    return isNaN(out) ? Number.MAX_SAFE_INTEGER : out
}

function orderRankComparator(a, b) {
    var aFinished = a.num_collected >= category.getTotalCollectibles(currentRaceCategory)
    var bFinished = b.num_collected >= category.getTotalCollectibles(currentRaceCategory)

    //If both finished, rank solely by time
    if (aFinished && bFinished) {
        return timeToSeconds(a.time) - timeToSeconds(b.time) || a.twitch_name.localeCompare(b.twitch_name)
    }
    if (aFinished) return -1 //Finishers always rank higher than non finishers
    if (bFinished) return 1

    var aQuit = a.status != "running"
    var bQuit = b.status != "running"

    //One quit, still sort by num collected, and if they're the same, in-progress runner goes first
    if (aQuit && !bQuit) {
        return b.num_collected - a.num_collected || 1
    }

    if (bQuit && !aQuit) {
        return b.num_collected - a.num_collected || -1
    }

    return b.num_collected - a.num_collected || timeToSeconds(a.estimate) - timeToSeconds(b.estimate) || a.twitch_name.localeCompare(b.twitch_name)
}

function orderDisplayComparator(a, b) {
    //Stubbing this function out because I still prefer this order, but Blood wants the other order. I'm undecided so I'll just 
    //keep the function as a comment
    return orderRankComparator(a, b)

    /*
    var aFinished = a.num_collected >= category.getTotalCollectibles(currentRaceCategory)
    var bFinished = b.num_collected >= category.getTotalCollectibles(currentRaceCategory)

    var aQuit = a.status !== "running"
    var bQuit = b.status !== "running"

    //If both players quit, num collectibles determines who gets shown first
    if (aQuit && bQuit) {
        return b.num_collected - a.num_collected
    }

    if (aQuit) return 1 //Quitters automatically get ranked lower than anybody else
    if (bQuit) return -1

    //Both finished. rank by finish time, faster time first
    if (aFinished && bFinished) {
    return timeToSeconds(a.time) - timeToSeconds(b.time) || a.twitch_name.localeCompare(b.twitch_name)
}

    //Finished player goes after the active one
    if (aFinished) return 1
    if (bFinished) return -1

    //Neither finished. rank by progress, most collected first
    return b.num_collected - a.num_collected || timeToSeconds(a.estimate) - timeToSeconds(b.estimate) || a.twitch_name.localeCompare(b.twitch_name)*/
}

function getPlacementMap(origPlayerRecords) {
    var placementMap = {}

    //Copy player records to not mutate the original
    var playerRecords = [...origPlayerRecords]

    playerRecords.sort(orderRankComparator)
    playerRecords.forEach((r, idx) => { 
        //0th index is place 1
        if (idx == 0) {
            placementMap[r.twitch_name] = idx + 1
            return
        }

        //Check the previous index to compare
        var prevPlayer = playerRecords[idx - 1]

        var prevFinished = prevPlayer.num_collected >= category.getTotalCollectibles(currentRaceCategory)
        var rFinished = r.num_collected >= category.getTotalCollectibles(currentRaceCategory)
    
        //If players are both finished, tie them if the finish times are the same, otherwise go by index
        if (prevFinished && rFinished) {
            if (prevPlayer.time === r.time) {
                placementMap[r.twitch_name] = placementMap[prevPlayer.twitch_name]
            } else {
                placementMap[r.twitch_name] = idx + 1
            }
            return
        }

        //If one player has quit, the quitter always is behind the current racer
        var prevQuit = prevPlayer.status != "running"
        var rQuit = r.status != "running"
        if (!prevQuit && rQuit) {
            //Previous is still going but you quit, you're behind
            placementMap[r.twitch_name] = idx + 1
            return
        }

        //Otherwise, go by num collected
        if (prevPlayer.num_collected == r.num_collected) {
            placementMap[r.twitch_name] = placementMap[prevPlayer.twitch_name]
            return
        }

        //Otherwise, go based on index
        placementMap[r.twitch_name] = idx + 1
    })

    return placementMap
}