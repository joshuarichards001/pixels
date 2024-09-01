package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/joho/godotenv"
)

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("error loading .env file")
	}
}

func validateIncomingMessage(update IncomingMessage) error {
	if update.Type != "update" {
		return fmt.Errorf("invalid message type: %s", update.Type)
	}

	if update.Data.Index < 0 || update.Data.Index >= 10000 {
		return fmt.Errorf("invalid index: %d", update.Data.Index)
	}

	colorNumber, err := strconv.Atoi(update.Data.Color)
	if err != nil {
		return fmt.Errorf("invalid color value: %s", update.Data.Color)
	}

	if colorNumber < 0 || colorNumber > 9 {
		return fmt.Errorf("color value out of range: %d", colorNumber)
	}

	return nil
}

func getIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}
