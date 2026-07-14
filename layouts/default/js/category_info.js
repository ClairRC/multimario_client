//This will hardcode known values for categories that currently exist. Probably not the most scalable but for now sufficient

const resourcesDirectory = "../resources/"

//Game Catgeory information
const sm64_120 = {"collectibles": 120, "background": "sm64_2.png", "icon": "sm64_progress_icon.png"}
const smg1_120 = {"collectibles": 120, "background": "smg1_2.png", "icon": "smg1_progress_icon.png"}
const sms_120 = {"collectibles": 120, "background": "sms_1.png", "icon": "sms_progress_icon.png"}
const smg2_242 = {"collectibles": 242, "background": "smg2_2.png", "icon": "smg1_progress_icon.png"}
const smo_all_moons = {"collectibles": 880, "background": "smo_1.png", "icon": "smo_progress_icon_moon.svg"}
const sm3dw_380 = {"collectibles": 380, "background": "sm3dw_2.png", "icon": "sm3dw_progress_icon.png"}

const sm64_70 = {"collectibles": 70, "background": "sm64_2.png", "icon": "sm64_progress_icon.png"}
const smg1_any = {"collectibles": 61, "background": "smg1_2.png", "icon": "smg1_progress_icon.png"}
const sms_any = {"collectibles": 44, "background": "sms_1.png", "icon": "sms_progress_icon.png"}
const smg2_any = {"collectibles": 71, "background": "smg2_2.png", "icon": "smg1_progress_icon.png"}
const smo_any = {"collectibles": 124, "background": "smo_1.png", "icon": "smo_progress_icon_moon.svg"}
const sm3dw_any = {"collectibles": 170, "background": "sm3dw_2.png", "icon": "sm3dw_progress_icon.png"}

//Race category information
const cat602 = {
    "total_collectibles": 602,
    "categories": [sm64_120, smg1_120, sms_120, smg2_242],
    "finish_background": "finish-smg2-242.png",
    "quit_background": "dnf.jpg"
}

const cat1120 = {
    "total_collectibles": 1120,
    "categories": [smo_all_moons, sms_120, sm64_120],
    "finish_background": "finish-smg2-242.png",
    "quit_background": "dnf.jpg"
}

const cat238 = {
    "total_collectibles": 238,
    "categories": [smo_any, sms_any, sm64_70],
    "finish_background": "finish-smg2-242.png",
    "quit_background": "dnf.jpg"
}

const cat246 = {
    "total_collectibles": 246,
    "categories": [sm64_70, smg1_any, sms_any, smg2_any],
    "finish_background": "finish-smg2-242.png",
    "quit_background": "dnf.jpg"
}

const cat540 = {
    "total_collectibles": 540,
    "categories": [smo_any, sm3dw_any, sm64_70, smg1_any, sms_any, smg2_any],
    "finish_background": "finish-smg2-242.png",
    "quit_background": "dnf.jpg"
}

export function getCurrentBackgroundImage(raceCategory, numCollectibles) {
    var startNumCollected = numCollectibles
    var cat = getRaceCategoryObj(raceCategory)

    if (cat === undefined) {
        return ""
    }

    for (let i = 0; i < cat.categories.length; i++) {
        if (startNumCollected >= cat.total_collectibles) {
            return resourcesDirectory + cat.finish_background
        } else if (startNumCollected >= cat.categories[i].collectibles) {
            startNumCollected -= cat.categories[i].collectibles
        } else {
            return resourcesDirectory + cat.categories[i].background
        }
    }
}

export function getDNFBackgroundImage(raceCategory) {
    var cat = getRaceCategoryObj(raceCategory)

    if (cat === undefined) {
        return ""
    }

    return resourcesDirectory + cat.quit_background
}

export function getGameCount(raceCategory, numCollectibles) {
    var startNumCollected = numCollectibles
    var cat = getRaceCategoryObj(raceCategory)

    if (cat === undefined) {
        return ""
    }

    for (let i = 0; i < cat.categories.length; i++) {
        if (startNumCollected >= cat.total_collectibles) {
            return cat.categories[cat.categories.length-1].collectibles
        } else if (startNumCollected >= cat.categories[i].collectibles) {
            startNumCollected -= cat.categories[i].collectibles
        } else {
            return startNumCollected
        }
    }
}

export function getTotalCollectibles(raceCategory) {
    var cat = getRaceCategoryObj(raceCategory)
    if (cat !== undefined) {
        return cat.total_collectibles
    }
}

export function getCurrentIconImage(raceCategory, numCollectibles) {
    var startNumCollected = numCollectibles
    var cat = getRaceCategoryObj(raceCategory)

    if (cat === undefined) {
        return ""
    }

    for (let i = 0; i < cat.categories.length; i++) {
        if (startNumCollected >= cat.total_collectibles) {
            return resourcesDirectory + cat.categories[cat.categories.length-1].icon
        } else if (startNumCollected >= cat.categories[i].collectibles) {
            startNumCollected -= cat.categories[i].collectibles
        } else {
            return resourcesDirectory + cat.categories[i].icon
        }
    }
}

function getRaceCategoryObj(raceCategoryName) {
    switch(raceCategoryName) {
        case "602":
            return cat602
        case "sandbox_100%":
            return cat1120
        case "246":
            return cat246
        case "sandbox_any%":
            return cat238
        case "540":
            return cat540
    }
}