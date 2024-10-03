package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"truenas_api/truenas_api"
)

// example usage
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide the TrueNAS server as an argument and the id to delete")
		os.Exit(1)
	}

	server := os.Args[1]
	id, _ := strconv.Atoi(os.Args[2])

	log.Printf("Connecting to TrueNAS server at %s", server)

	serverURL := "ws://" + server + "/api/current"

	client, err := truenas_api.NewClient(serverURL, false)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	username := os.Getenv("TRUENAS_USERNAME")
	password := os.Getenv("TRUENAS_PASSWORD")
	apiKey := os.Getenv("TRUENAS_API_KEY")

	// Logging in with username/password or API key.
	if err := client.Login(username, password, apiKey); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	log.Println("Login successful!")

	client.Ping()

	// The params are wrapped in an array inside the Call function
	res, err := client.Call("user.delete", 10, []interface{}{id})
	if err != nil {
		log.Fatalf("failed to delete user: %v", err)
	}

	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, res, "", "\t")
	log.Printf("Result: %s", prettyJSON.String())

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
