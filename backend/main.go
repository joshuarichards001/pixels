package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	loadEnv()
	server := newServer()
	go server.run(ctx)

	http.HandleFunc("/ws", server.handleConnections)
	http.HandleFunc("/pixels", corsMiddleware(server.handleGetPixels))

	fmt.Println("Server is running on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
