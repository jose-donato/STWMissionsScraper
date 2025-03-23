package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gocolly/colly/v2"
)

// VBucksMission represents a mission that rewards V-Bucks
type VBucksMission struct {
	Area        string
	PowerLevel  string
	Amount      string
	MissionType string
}

func main() {
	// Create a new collector
	c := colly.NewCollector()

	// Create a slice to store V-Bucks missions
	var vbucksMissions []VBucksMission

	// Look for divs containing V-Bucks missions
	c.OnHTML("div.news-link div.infonotice", func(e *colly.HTMLElement) {
		// Skip the support-a-creator div
		if strings.Contains(e.Text, "Use code \"iFeral\"") {
			return
		}

		text := strings.TrimSpace(e.Text)

		// Split by "in" to get the area
		parts := strings.Split(text, " in ")
		if len(parts) < 2 {
			return
		}

		area := strings.TrimSpace(parts[1])
		mainPart := parts[0]

		// Split the main part by spaces
		fields := strings.Fields(mainPart)
		if len(fields) < 2 {
			return
		}

		// First field is amount, second is power level, rest is mission type
		amount := fields[0]
		powerLevel := fields[1]

		// Check if power level has other text attached
		powerLevelDigits := ""
		missionType := ""

		for i, c := range powerLevel {
			if c >= '0' && c <= '9' {
				powerLevelDigits += string(c)
			} else {
				// Once we hit non-digits, the rest is part of the mission type
				missionType = powerLevel[i:] + " " + strings.Join(fields[2:], " ")
				break
			}
		}

		// If we didn't find any non-digits, then the mission type is just the remaining fields
		if missionType == "" {
			missionType = strings.Join(fields[2:], " ")
		}

		mission := VBucksMission{
			Amount:      amount,
			PowerLevel:  powerLevelDigits,
			MissionType: strings.TrimSpace(missionType),
			Area:        area,
		}

		vbucksMissions = append(vbucksMissions, mission)
	})

	// Start the scraping process
	err := c.Visit("https://freethevbucks.com/timed-missions/")
	if err != nil {
		log.Fatal(err)
	}

	// Only print the mission details table
	if len(vbucksMissions) > 0 {
		// Print just the table without any title
		fmt.Println("| Area | Power Level | V-Bucks | Mission Type |")
		fmt.Println("|------|-------------|---------|--------------|")

		for _, mission := range vbucksMissions {
			fmt.Printf("| %s | %s | %s | %s |\n",
				mission.Area,
				mission.PowerLevel,
				mission.Amount,
				mission.MissionType)
		}
	} else {
		fmt.Println("| Area | Power Level | V-Bucks | Mission Type |")
		fmt.Println("|------|-------------|---------|--------------|")
		fmt.Println("| No V-Bucks missions found today | - | - | - |")
	}
}
