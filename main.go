package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
)

// VBucksMission represents a mission that rewards V-Bucks
type VBucksMission struct {
	Area        string
	PowerLevel  string
	Amount      string
	MissionType string
}

// CacheData represents the data we'll be caching
type CacheData struct {
	Timestamp      time.Time
	VBucksMissions []VBucksMission
}

// File paths
const (
	cacheFile = "vbucks_cache.json"
	envFile   = ".env"
)

func main() {
	// Load environment variables from .env file
	err := loadEnv()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get bot token from environment
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set in .env file")
	}

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Start listening for updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Handle updates in a separate goroutine
	go func() {
		for update := range updates {
			if update.Message == nil {
				continue
			}

			// Log the chat ID for setup purposes
			log.Printf("Received message from chat ID: %d", update.Message.Chat.ID)

			// Process commands
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start":
					// Send welcome message and show missions
					welcomeMsg := "Welcome to the Fortnite V-Bucks Missions Bot!\n\n" +
						"This bot will notify you of daily V-Bucks missions in Fortnite Save the World.\n\n" +
						"Here are today's missions:"
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMsg)
					bot.Send(msg)

					// Send V-Bucks missions
					missions := getMissions()
					vbucksMsg := tgbotapi.NewMessage(update.Message.Chat.ID, formatMissionsForTelegram(missions))
					vbucksMsg.ParseMode = "MarkdownV2"
					bot.Send(vbucksMsg)
				case "vbucks":
					// Get missions and send as a message
					missions := getMissions()
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, formatMissionsForTelegram(missions))
					msg.ParseMode = "MarkdownV2"
					bot.Send(msg)
				case "help":
					helpText := "Available commands:\n" +
						"/vbucks - Show today's V-Bucks missions\n" +
						"/help - Show this help message"
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
					bot.Send(msg)
				default:
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command. Try /help")
					bot.Send(msg)
				}
			}
		}
	}()

	// Keep the program running
	select {}
}

// loadEnv loads environment variables from .env file
func loadEnv() error {
	// Check if .env file exists
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// Create a default .env file
		defaultEnv := `# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_bot_token_here
`
		if err := ioutil.WriteFile(envFile, []byte(defaultEnv), 0644); err != nil {
			return fmt.Errorf("failed to create default .env file: %v", err)
		}
		return fmt.Errorf("please edit %s with your Telegram bot token", envFile)
	}

	// Load the .env file
	return godotenv.Load(envFile)
}

// getMissions gets missions, using the cache if valid
func getMissions() []VBucksMission {
	var vbucksMissions []VBucksMission

	// Try to load from cache first
	if cachedData, cacheValid := loadFromCache(); cacheValid {
		vbucksMissions = cachedData.VBucksMissions
	} else {
		// If cache is invalid or doesn't exist, fetch new data
		vbucksMissions = fetchMissions()

		// Save the new data to cache
		saveToCache(vbucksMissions)
	}

	return vbucksMissions
}

// fetchMissions scrapes the website for V-Bucks missions
func fetchMissions() []VBucksMission {
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

	return vbucksMissions
}

// formatMissionsForTelegram formats the missions as a markdown table for Telegram
// Note: We're using MarkdownV2 which requires escaping special characters
func formatMissionsForTelegram(vbucksMissions []VBucksMission) string {
	var result strings.Builder

	if len(vbucksMissions) > 0 {
		result.WriteString("*V\\-Bucks Missions Today*\n\n")

		// Simple list format instead of table (tables are hard to format in Telegram)
		for i, mission := range vbucksMissions {
			result.WriteString(fmt.Sprintf("%d\\. PL %s %s in %s \\- *%s V\\-Bucks*\n",
				i+1,
				escapeMarkdown(mission.PowerLevel),
				escapeMarkdown(mission.MissionType),
				escapeMarkdown(mission.Area),
				escapeMarkdown(mission.Amount),
			))
		}

		// Calculate total
		total := 0
		for _, mission := range vbucksMissions {
			amount, _ := strconv.Atoi(mission.Amount)
			total += amount
		}

		result.WriteString(fmt.Sprintf("\n*Total: %d V\\-Bucks*", total))
	} else {
		result.WriteString("*No V\\-Bucks missions found today*")
	}

	return result.String()
}

// escapeMarkdown escapes special characters for Telegram's MarkdownV2 format
func escapeMarkdown(text string) string {
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range specialChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}
	return text
}

// loadFromCache tries to load missions from the cache file
// Returns the cached data and a boolean indicating if the cache is valid
func loadFromCache() (CacheData, bool) {
	var cacheData CacheData

	// Check if cache file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return cacheData, false
	}

	// Read cache file
	data, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		log.Printf("Error reading cache file: %v", err)
		return cacheData, false
	}

	// Parse JSON data
	if err := json.Unmarshal(data, &cacheData); err != nil {
		log.Printf("Error parsing cache file: %v", err)
		return cacheData, false
	}

	// Check if cache is still valid
	// Cache is valid if it's from today after 00:10 UTC
	now := time.Now().UTC()
	cacheTime := cacheData.Timestamp

	// Determine if we're past 00:10 UTC today
	todayReset := time.Date(now.Year(), now.Month(), now.Day(), 0, 10, 0, 0, time.UTC)

	// Cache is valid if:
	// 1. Cache timestamp is after today's reset
	// 2. Current time is also after today's reset
	cacheValid := cacheTime.After(todayReset) && now.After(todayReset) &&
		cacheTime.Year() == now.Year() &&
		cacheTime.Month() == now.Month() &&
		cacheTime.Day() == now.Day()

	return cacheData, cacheValid
}

// saveToCache saves the missions data to the cache file
func saveToCache(missions []VBucksMission) {
	cacheData := CacheData{
		Timestamp:      time.Now().UTC(),
		VBucksMissions: missions,
	}

	// Convert to JSON
	data, err := json.Marshal(cacheData)
	if err != nil {
		log.Printf("Error creating cache data: %v", err)
		return
	}

	// Write to file
	if err := ioutil.WriteFile(cacheFile, data, 0644); err != nil {
		log.Printf("Error writing cache file: %v", err)
	}
}
