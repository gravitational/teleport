// This example program demonstrates how to connect to a Postgres database
// using certificates issued by Teleport Machine ID.

package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v4/stdlib"
)

func main() {
	// Open connection to database.
	db, err := sql.Open("pgx", fmt.Sprint(
		"host=localhost ",
		"port=1234 ",
		"dbname=example ",
		"user=alice ",
		// The next four options should be omitted if the local proxy has been
		// placed in "authenticated tunnel" mode.
		"sslmode=verify-full ",
		"sslrootcert=/opt/machine-id/teleport-host-ca.crt ",
		"sslkey=/opt/machine-id/key ",
		"sslcert=/opt/machine-id/tlscert ",
	))
	if err != nil {
		log.Fatalf("Failed to open database: %v.", err)
	}

	defer db.Close()

	// Call "Ping" to test connectivity.
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to Ping database: %v.", err)
	}

	log.Printf("Successfully connected to PostgreSQL.")
}
