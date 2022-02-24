package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/meinside/telegram-totp-bot/bot"
)

// config struct for this bot
type config struct {
	TelegramBotToken     string `json:"telegram_bot_token"`
	DatabaseFileLocation string `json:"database_file_location"`
}

func main() {
	configFilepath := flag.String("config", "", "config file's path (eg. /home/ubuntu/bot.json)")
	flag.Parse()

	if configFilepath == nil || *configFilepath == "" {
		printUsageAndExit()
	}

	bytes, err := os.ReadFile(*configFilepath)
	if err != nil {
		log.Fatalf("Failed to read config file: %s", err)
	}

	var conf config
	if err = json.Unmarshal(bytes, &conf); err != nil {
		log.Fatalf("Failed to parse config file: %s", err)
	}

	// start running the bot
	bot.Run(conf.TelegramBotToken, conf.DatabaseFileLocation)
}

func printUsageAndExit() {
	fmt.Println("Usage:")

	flag.PrintDefaults()

	os.Exit(1)
}
