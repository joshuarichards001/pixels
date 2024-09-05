package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("error loading .env file")
	}
}

func verifyHCaptcha(token string) error {
	values := url.Values{
		"response": {token},
		"secret":   {os.Getenv("HCAPTCHA_SECRET")},
	}

	resp, err := http.PostForm("https://hcaptcha.com/siteverify", values)

	if err != nil {
		return fmt.Errorf("error verifying hCaptcha: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool     `json:"success"`
		Errors  []string `json:"error-codes"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding hCaptcha response: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("hCaptcha verification failed, Errors: %+v", result.Errors)
	}

	return nil
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
	if ip := r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}

	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}

	return r.RemoteAddr
}
