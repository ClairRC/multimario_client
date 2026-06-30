package client

//This package will contain functionality for interacting with the Multimario API
//TODO: This package is unnecessary. The mmapi package is what I should be using. Delete this when you get the chance.

//Type for clients that interact with the backend.
//This is an interface so that we can do mock API calls to mimic the backend for development purposes
type MMClient interface {
	GetRacerTwitchNames() []string
}

//Client structs
type TestClient struct {} //For testing/dev
type Client struct {} //Calls backend multimario API to get/send updated info

//TODO Set the default to Not be the test client
var DefaultMMClient MMClient = TestClient{}

//Test client implementation
func (TestClient) GetRacerTwitchNames() []string { 
	//Make up some Twitch users and mimic API response
	//For now, I am using the people signed up for the upcoming 602 race
	players := []string{
		"Odme_",
		"NathanCarter602",
		"Rodillo_",
		"Galaxtic",
		"zGamuT",
		"fizz64",
		"galax_v",
		"serving_hunter",
		"Muimania",
		"CheesySRC",
		"BloodDerg",
		"jukatox",
		"AnOrdinaryPerson",
		"KasualPlayz",
		"calumj28",
		"serpals",
		"Bird650",
		"SNRobi",
		"Zakart_Qc",
		"Zypher11_",
		"wookis_",
		"LunaEclipse_4",
		"Aurelia400",
		"devin_2319",
		"Zans64",
		"Nostalgia64runs",
		"michaelsspeedruns",
		"Wartartar",
		"LucineSR",
		"KingToad74EE",
		"ShardOfKingdoms",
		"PinShark2112",
		"Sintrill",
		"imchach",
		"Welldone11",
	}
	return players
}