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

func runHTTPServer(ctx context.Context) error {
	server := &http.Server{
		Addr: ":8080",
	}

	shutdown := make(chan error, 1)
	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Println("Server is shutting down")
		shutdown <- server.Shutdown(ctx)
	}()

	fmt.Println("Server is running on :8080")
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen and serve: %w", err)
	}

	err = <-shutdown
	if err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	loadEnv()
	server := NewServer()
	go server.Run(ctx)

	http.HandleFunc("/ws", server.handleConnections)
	http.HandleFunc("/pixels", corsMiddleware(server.handleGetPixels))

	err := runHTTPServer(ctx)
	if err != nil {
		log.Fatal("Error running HTTP server: ", err)
	}
}
