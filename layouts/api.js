//This file includes helpful API functions for communicating with the bot.

//Listeners for different types of API calls
const listeners = {
    init: [],
    update: [],
}

//Setup API stuff
setup()
async function setup() {
    await setupSSE()
}

//Get updates from bot via SSE
async function setupSSE() {
    //Setup SSE
    const eventSource = new EventSource("/api/events")
    eventSource.onmessage = (event) => {
        handleSSEMessage(event.data)
    }
    eventSource.onerror = (e) => console.error("SSE error", e)
}

//Subscribes a function to be called with Init data
export function onInit(fn) {
    listeners.init.push(fn)
}

//Subscribes a function to be called with Update data
export function onUpdate(fn) {
    listeners.update.push(fn)
}

//Function that calls each subscribed function with passed in race info
async function handleInitData(initData) {
    listeners.init.forEach(listenerFn => {
        listenerFn(initData)
    });
}

async function handleUpdateData(updateData) {
    listeners.update.forEach(listenerFn => {
        listenerFn(updateData)
    });
}

//Pass in the message string and parse it to JSON object and pass it off from there
async function handleSSEMessage(data) {
    const dataObj = JSON.parse(data)

    //Check whether this is an init or an update
    if (dataObj.init !== undefined) {
        await handleInitData(dataObj.init)
    } else if (dataObj.update !== undefined) {
        await handleUpdateData(dataObj.update)
    }
}