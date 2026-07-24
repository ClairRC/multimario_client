package categoryinfo

import "strings"

//This package just has useful functions for getting certain category-specific information

type GameCategory struct {
	GameName string
	Name string
	NumCollectibles int
	CollectibleType string
}

type Category struct {
	Name string
	TotalNumCollectibles int
	Games []GameCategory
}

var sm64_120 GameCategory = GameCategory{"SM64", "sm64_120", 120, "Stars"}
var smg1_120 GameCategory = GameCategory{"SMG1", "smg1_120", 120, "Stars"}
var sms_120 GameCategory = GameCategory{"SMS", "sms_120", 120, "Shines"}
var smg2_242 GameCategory = GameCategory{"SMG2", "smg2_242", 242, "Stars"}
var smo_all_moons GameCategory = GameCategory{"SMO", "smo_all_moons", 880, "Moons"}
var sm3dw_380 GameCategory = GameCategory{"SM3DW", "sm3dw_380", 380, "Green Stars"}

var sm64_70 GameCategory = GameCategory{"SM64", "sm64_70", 70, "Stars"}
var smg1_any GameCategory = GameCategory{"SMG1", "smg1_any", 61, "Stars"}
var sms_any GameCategory = GameCategory{"SMS", "sms_any", 44, "Shines"}
var smg2_any GameCategory = GameCategory{"SMG2", "smg2_any", 71, "Stars"}
var smo_any GameCategory = GameCategory{"SMO", "smo_any", 124, "Moons"}
var sm3dw_any GameCategory = GameCategory{"SM3DW", "sm3dw_any", 170, "Green Stars"}

var categories = map[string]Category {
	"602": {"602", 602, []GameCategory{sm64_120, smg1_120, sms_120, smg2_242}},
	"246": {"246", 246, []GameCategory{sm64_70, smg1_any, sms_any, smg2_any}},
	"sandbox_any%": {"sandbox_any%", 238, []GameCategory{smo_any, sms_any, sm64_70}},
	"sandbox_100%": {"sandbox_100%", 1120, []GameCategory{smo_all_moons, sms_120, sm64_120}},
	"540": {"540", 540, []GameCategory{smo_any, sm3dw_any, sm64_70, smg1_any, sms_any, smg2_any}},
}

func GetCollectibleType(category string, numCollectibles int) string {
	//Get this category
	cat := categories[category]

	//Category doesn't exist, default to collectibles
	if cat.Name == "" {
		return "Collectibles"
	}

	//Loop through it's game categories till we find the right now
	amount := numCollectibles
	var game GameCategory
	for _, g := range cat.Games {
		if amount > g.NumCollectibles {
			amount -= g.NumCollectibles
		} else {
			game = g
			break
		}
	}

	return game.CollectibleType
}

func CurrentGameName(category string, numCollectibles int) string {
	//Get this category
	cat := categories[category]

	//Category doesn't exist, default to collectibles
	if cat.Name == "" {
		return ""
	}

	//Loop through it's game categories till we find the right now
	amount := numCollectibles
	var game GameCategory
	for _, g := range cat.Games {
		if amount > g.NumCollectibles {
			amount -= g.NumCollectibles
		} else {
			game = g
			break
		}
	}

	return game.GameName
}

func GetGameProgress(category string, numCollectibles int) int {
	//Get this category
	cat := categories[category]

	//Category doesn't exist, default to collectibles
	if cat.Name == "" {
		return -1
	}

	//Loop through it's game categories till we find the right now
	amount := numCollectibles
	for _, g := range cat.Games {
		if amount > g.NumCollectibles {
			amount -= g.NumCollectibles
		} else {
			break
		}
	}

	return amount
}

func GetGameCategoryFromGameName(raceCategory string, gameName string) string {
	cat, ok := categories[raceCategory]
	if !ok {
		return ""
	}

	var gameCategory string = ""
	for _, g := range cat.Games {
		if strings.EqualFold(g.GameName, gameName) {
			gameCategory = g.Name
		}
	}

	return gameCategory
}

func GetTotalCollectiblesFromCategoryName(raceCategory string) int {
	cat, ok := categories[raceCategory]
	if !ok {
		return -1
	}

	return cat.TotalNumCollectibles
}