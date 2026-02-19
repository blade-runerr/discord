package main

import (
	"log"
	"os"

	"mini-discord/internal/httpserver"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	srv, err := httpserver.NewServer(":" + port)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Server listening on http://localhost:%s", port)
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
