package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"truenas_api/truenas_api"
)

// Define the structures to parse the JSON response.
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

func main() {
	// Checking the command-line arguments.
	if len(os.Args) < 2 {
		log.Fatal("Usage: app_upgrade <server>")
	}

	server := os.Args[1]
	log.Printf("Connecting to TrueNAS server at %s", server)
	serverURL := "ws://" + server + "/api/current"

	// Creating the TrueNAS API client.
	client, err := truenas_api.NewClient(serverURL, false)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
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

	if err := client.SubscribeToJobs(); err != nil {
		log.Fatalf("Failed to subscribe to job updates: %v", err)
	}

	// Making the API call.
	response, err := client.Call("app.query", 10, []interface{}{})
	if err != nil {
		log.Fatalf("Failed to update apps: %v", err)
	}

	// Parsing the JSON-RPC response.
	var rpcResponse JSONRPCResponse
	if err := json.Unmarshal(response, &rpcResponse); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Printing the parsed data.
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
