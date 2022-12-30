// This example program demonstrates how to connect to a MongoDB database
// using certificates issued by Teleport Machine ID.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create client and connect to MongoDB. Make sure to modify the host,
	// port, and certificate paths.
	uri := fmt.Sprintf(
		"mongodb://localhost:1234/?tlsCAFile=%s&tlsCertificateKeyFile=%s",
		"/opt/machine-id/mongo.cas",
		"/opt/machine-id/mongo.crt",
	)
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Failed to create database client: %v.", err)
	}
	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v.", err)
	}

	defer client.Disconnect(ctx)

	log.Printf("Successfully connected to MongoDB.")

	// List databases to test connectivity.
	databases, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to list databases: %v.", err)
	}
	log.Println(databases)

}
