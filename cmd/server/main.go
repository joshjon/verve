package main

import (
	"log"

	"verve/internal/server"
)

func main() {
	log.Println("Starting Verve API server...")

	srv := server.New()
	if err := srv.Start(":8080"); err != nil {
		log.Fatal(err)
	}
}
