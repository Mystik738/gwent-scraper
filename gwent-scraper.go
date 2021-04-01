package main

import (
	"encoding/csv"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	IdFile      = "Players.csv"
	OutputFile  = "Data.csv"
	Concurrency = 8
	UrlBase     = "https://www.playgwent.com/en/profile/"
	Debug       = false
)

type Player struct {
	Current  ProfileData
	Total    ProfileData
	Losses   int
	Draws    int
	MMR      int
	Rank     int
	Prestige int
	Level    int
	Id       string
}

type ProfileData struct {
	Overall  int       `json:"overall"`
	Factions []Faction `json:"factions"`
}

type Faction struct {
	Slug  string `json:"slug"`
	Count int    `json:"count"`
}

func NewPlayer(name string) Player {
	p := Player{}
	p.Id = name
	p.Rank = 30

	return p
}

func main() {
	log.SetOutput(os.Stderr)
	players := make([]Player, 0)
	sem := make(chan bool, Concurrency)

	//Regexs for parsing the pages
	privateRegex := regexp.MustCompile(`THIS PLAYER PROFILE IS PRIVATE`)
	prestigeRegex := regexp.MustCompile(`prestige--(?P<prestige>[0-9]*)"><strong>[\s]*(?P<level>[0-9]*)`)
	winsRegex := regexp.MustCompile(`var profileDataWins = (?P<json>.*?);`)
	lossesRegex := regexp.MustCompile(`Losses</td><td>(?P<matches>[0-9,]*) matches</td>`)
	drawsRegex := regexp.MustCompile(`Draws</td><td>(?P<matches>[0-9,]*) matches</td>`)
	currentRegex := regexp.MustCompile(`var profileDataCurrent = (?P<json>.*?);`)
	mmrRegex := regexp.MustCompile(`(?P<mmr>[0-9][0-9,]*) MMR`)
	rankRegex := regexp.MustCompile(`-details__rank"><strong>(?P<rank>[0-9]*)<`)

	//Load Ids from file
	f, err := os.Open(IdFile)
	checkError(err)
	defer f.Close()

	lines, err := csv.NewReader(f).ReadAll()
	checkError(err)
	for _, line := range lines {
		players = append(players, NewPlayer(line[0]))
	}

	//Do we need a mutex here? We're writing to distinct players..
	for i, _ := range players {
		sem <- true
		go func(p *Player) {
			defer func() { <-sem }()

			resp, err := http.Get(UrlBase + p.Id)
			checkError(err)
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				checkError(err)
				bodyString := string(bodyBytes)
				//Check for private
				matched := privateRegex.MatchString(bodyString)
				if matched {
					if Debug {
						log.Println(p.Id + "'s profile is private")
					}
				} else {
					//Get stats
					winsMatch := winsRegex.FindStringSubmatch(bodyString)
					err = json.Unmarshal([]byte(winsMatch[1]), &p.Total)
					checkError(err)
					currentMatch := currentRegex.FindStringSubmatch(bodyString)
					err = json.Unmarshal([]byte(currentMatch[1]), &p.Current)
					mmrMatch := mmrRegex.FindStringSubmatch(bodyString)
					prestigeMatch := prestigeRegex.FindStringSubmatch(bodyString)
					checkError(err)
					if len(mmrMatch) > 1 {
						p.MMR, err = strconv.Atoi(strings.ReplaceAll(mmrMatch[1], ",", ""))
						checkError(err)
					}
					lossesMatch := lossesRegex.FindStringSubmatch(bodyString)
					if len(mmrMatch) > 1 {
						p.Losses, err = strconv.Atoi(strings.ReplaceAll(lossesMatch[1], ",", ""))
						checkError(err)
					}
					drawsMatch := drawsRegex.FindStringSubmatch(bodyString)
					if len(mmrMatch) > 1 {
						p.Draws, err = strconv.Atoi(strings.ReplaceAll(drawsMatch[1], ",", ""))
						checkError(err)
					}
					rankMatch := rankRegex.FindStringSubmatch(bodyString)
					if len(mmrMatch) > 1 {
						p.Rank, err = strconv.Atoi(rankMatch[1])
						checkError(err)
					}
					if len(prestigeMatch) > 1 {
						p.Prestige, err = strconv.Atoi(prestigeMatch[1])
						checkError(err)
						p.Level, err = strconv.Atoi(prestigeMatch[2])
						checkError(err)
					}

					if Debug {
						log.Println(p.Id, ":", "Rank:", p.Rank, "All:", p.Total.Overall, "Wins:", p.Current.Overall, "Losses:", p.Losses, "Draws:", p.Draws, "MMR:", p.MMR)
					}
				}
			}

		}(&players[i])
	}

	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	file, err := os.Create(OutputFile)
	checkError(err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	//There is more data than this but currently this is all that's relevant
	err = writer.Write([]string{"id", "rank", "total wins", "current wins", "current losses", "current draws", "MMR", "prestige", "level"})
	checkError(err)

	for _, p := range players {
		err := writer.Write([]string{p.Id, strconv.Itoa(p.Rank), strconv.Itoa(p.Total.Overall), strconv.Itoa(p.Current.Overall), strconv.Itoa(p.Losses), strconv.Itoa(p.Draws), strconv.Itoa(p.MMR), strconv.Itoa(p.Prestige), strconv.Itoa(p.Level)})
		checkError(err)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
