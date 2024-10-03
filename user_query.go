package main

import (
	"log"
	"os"
	"truenas_api/truenas_api"
)

// example usage
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide the TrueNAS server as an argument")
		os.Exit(1)
	}

	server := os.Args[1]

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

	// Example call to query a user
	params := []interface{}{}

	// The params are wrapped in an array inside the Call function
	res, err := client.Call("user.query", 10, []interface{}{params})
	if err != nil {
		log.Fatalf("failed to query user: %v", err)
	}
	log.Printf("%s", res)

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
