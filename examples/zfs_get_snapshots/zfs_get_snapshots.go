package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"truenas_api/truenas_api"
)

// example usage
func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: zfs_get_snapshot <server> <dataset>")
		os.Exit(1)
	}

	server := os.Args[1]
	dataset := os.Args[2]

	log.Printf("Connecting to TrueNAS server at %s", server)

	serverURL := "wss://" + server + "/api/current"

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

	// client.Ping()

	// Define the query_filters as a list of strings
	queryFilters := []string{"dataset", "=", dataset}

	// Convert the []string to []interface{}
	interfaceList := make([]interface{}, len(queryFilters))
	for i, v := range queryFilters {
		interfaceList[i] = v
	}

	// Directly pass the list as a parameter to Call
	params := []interface{}{
		interfaceList,
	}

	job, err := client.Call("zfs.snapshot.query", 200, []interface{}{params})
	if err != nil {
		log.Fatalf("failed to query snapshots: %v", err)
	}
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, job, "", "\t")
	log.Printf("Dataset Snapshots: %s", prettyJSON.String())

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
