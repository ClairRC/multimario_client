import { onInit, onUpdate } from '../api.js'

onInit(logInitValue)

function logInitValue(data) {
    data.records.forEach(playerData => {
        console.log(playerData.player_name)
    });
}