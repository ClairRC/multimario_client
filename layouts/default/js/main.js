import { onInit, onUpdate } from '../api.js'
import * as player from './player_card.js'
import * as category from './category_info.js'

onInit((data) => {
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
        var card = player.getPlayerCard(record.player_name, record.twitch_name, 1, pictureURL, record.num_collected, data.race_category)
        var isInTop3 = i <= 2
        
        if (isInTop3) {
            top3.innerHTML += card
        } else {
            statsGrid.innerHTML += card
        }
    });

    document.fonts.ready.then(() => {
        fixTextSizing()
    })
})

onUpdate((data) => {
    
})


function logInitValue(data) {
    data.records.forEach(playerData => {
        console.log(playerData.player_name)
    });
}

function fixTextSizing() {
    //Get user name elements to fit them properly
    var userNames = document.querySelectorAll(".user-name")
    userNames.forEach(element => {
        player.fitText(element, 1.7, 0.8)
    });

    var progressValues = document.querySelectorAll(".progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.5, 1.2)
    });

    //Total progress needs to have a smaller max size
    progressValues = document.querySelectorAll(".numeric-progress")
    progressValues.forEach(element => {
        player.fitText(element, 1.2, 0.8)
    });
}