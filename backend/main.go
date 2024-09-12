package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func runHTTPServer(ctx context.Context) {
	server := &http.Server{
		Addr: ":8080",
	}

	shutdown := make(chan error, 1)
	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		shutdown <- server.Shutdown(ctx)
	}()

	fmt.Println("Server is running on :8080")
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("ListenAndServe: ", err)
	}

	err = <-shutdown
	if err != nil {
		log.Fatal("Shutdown: ", err)
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	loadEnv()
	server := newServer()
	go server.run(ctx)

	http.HandleFunc("/ws", server.handleConnections)
	http.HandleFunc("/pixels", corsMiddleware(server.handleGetPixels))
	runHTTPServer(ctx)
}
