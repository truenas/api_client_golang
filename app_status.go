package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"truenas_api/truenas_api"
)

// Define the structures to parse the JSON response
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  []App  `json:"result"`
	ID      int    `json:"id"`
}

type App struct {
	Name            string          `json:"name"`
	ID              string          `json:"id"`
	State           string          `json:"state"`
	ActiveWorkloads ActiveWorkloads `json:"active_workloads"`
	Metadata        Metadata        `json:"metadata"`
}

type ActiveWorkloads struct {
	Containers       int               `json:"containers"`
	ContainerDetails []ContainerDetail `json:"container_details"`
}

type ContainerDetail struct {
	ServiceName string `json:"service_name"`
	Image       string `json:"image"`
	State       string `json:"state"`
}

type Metadata struct {
	AppVersion string `json:"app_version"`
}

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

	// Directly pass the list as a parameter to Call
	params := []interface{}{}

	// Subscribe to job updates
	if err := client.SubscribeToJobs(); err != nil {
		log.Fatalf("failed to subscribe to job updates: %v", err)
	}

	response, err := client.Call("app.query", 10, []interface{}{params})
	if err != nil {
		log.Fatalf("failed to update apps: %v", err)
	}

	// Parse the JSON-RPC response
	var rpcResponse JSONRPCResponse
	err = json.Unmarshal(response, &rpcResponse)
	if err != nil {
		log.Fatalf("failed to unmarshal response: %v", err)
	}

	// Print the parsed data
	for _, app := range rpcResponse.Result {
		fmt.Printf("App Name: %s\n", app.Name)
		fmt.Printf("App ID: %s\n", app.ID)
		fmt.Printf("State: %s\n", app.State)
		fmt.Printf("Containers: %d\n", app.ActiveWorkloads.Containers)
		for _, container := range app.ActiveWorkloads.ContainerDetails {
			fmt.Printf("  Service Name: %s\n", container.ServiceName)
			fmt.Printf("  Image: %s\n", container.Image)
			fmt.Printf("  State: %s\n", container.State)
		}
		fmt.Printf("App Version: %s\n", app.Metadata.AppVersion)
		fmt.Println()
	}

	log.Println("Client closed.")
}
