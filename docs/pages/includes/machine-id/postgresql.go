package main

import (
	"database/sql"
	"log"
)

func main() {
	// Open connection to database.
	db, err := sql.Open("postgres", "host=localhost")
	if err != nil {
		log.Fatalf("Failed to Open database: %v.", err)
	}
	defer db.Close()

	// Call "Ping" to test connectivity.
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to Ping database: %v.", err)
	}

	log.Printf("Successfully connected to PostgreSQL.")
}
