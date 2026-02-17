package main

import (
	"log"

	"mini-discord/internal/httpserver"
)

func main() {
	srv, err := httpserver.NewServer(":8000")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Server listening on http://localhost:8000")
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
