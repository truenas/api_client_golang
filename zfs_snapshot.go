package main

import (
	"log"
	"os"
	"truenas_api/truenas_api"
)

// example usage
func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: zfs_snapshot <server> <dataset> <snapshot_name>")
		os.Exit(1)
	}

	server := os.Args[1]
	dataset := os.Args[2]
	snapshot_name := os.Args[3]

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
	// Example call to create a user
	params := map[string]interface{}{
		"dataset": dataset,
		"name":    snapshot_name,
	}

	// The params are wrapped in an array inside the Call function
	res, err := client.Call("zfs.snapshot.create", 10, []interface{}{params})
	if err != nil {
		log.Fatalf("failed to snapshot user: %v", err)
	}
	log.Printf("Dataset snapshotted: %s", res)

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
