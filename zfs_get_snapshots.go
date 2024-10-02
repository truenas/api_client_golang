package main

import (
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

	serverURL := "ws://" + server + "/api/current"

	client, err := truenas_api.NewClient(serverURL)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Example login with username and password
	username := ""
	password := ""
	apiKey := "" // Leave empty if using username/password
	//apiKey := ""

	err = client.Login(username, password, apiKey)
	if err != nil {
		log.Fatalf("login failed: %v", err)
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
		log.Fatalf("failed to snapshot user: %v", err)
	}

	log.Printf("Dataset snapshots: %v", string(job))

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
