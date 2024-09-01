package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	loadEnv()
	server := newServer()
	go server.run()

	http.HandleFunc("/ws", server.handleConnections)

	fmt.Println("Server is running on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
