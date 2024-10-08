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
		log.Fatal("Usage: apy_key <server> <api_key_name>")
		os.Exit(1)
	}

	server := os.Args[1]
	new_api_key_name := os.Args[2]

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

	// Example call to create a user
	//       payload = {'name': API_KEY_NAME, 'allowlist': [{'resource': '*', 'method': '*'}]}
	params2 := map[string]interface{}{
		"resource": "*",
		"method":   "*",
	}

	params := map[string]interface{}{
		"name":      new_api_key_name,
		"allowlist": []interface{}{params2},
	}

	// The params are wrapped in an array inside the Call function
	res, err := client.Call("api_key.create", 10, []interface{}{params})
	if err != nil {
		log.Fatalf("failed to create api_key: %v", err)
	}
	//log.Printf("api_key created: %s", res)
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, res, "", "\t")
	log.Printf("%s", prettyJSON.String())

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
