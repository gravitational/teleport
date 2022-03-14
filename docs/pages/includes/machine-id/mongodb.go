package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create client and connect to MongoDB.
	client, err := mongo.NewClient(options.Client().ApplyURI("<MONGODB_URI>"))
	if err != nil {
		log.Fatalf("Failed to create database client: %v.", err)
	}
	defer client.Disconnect(ctx)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v.", err)
	}

	// List databases to test connectivity.
	databases, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to list databases: %v.", err)
	}
	log.Println(databases)

	log.Printf("Successfully connected to MongoDB.")
}
