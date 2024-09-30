package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	client, err := truenas_api.NewClient(serverURL)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Example login with username and password
	username := ""
	password := ""
	apiKey := "3-gd8JSwGbWoTiJdEXRHU9yRp5oke0KFYAZhUZPTjFKOiWjtKscdFodEDLNncwXbUe" // Leave empty if using username/password
	//apiKey := ""

	err = client.Login(username, password, apiKey)
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	log.Println("Login successful!")

	pres, perr := client.Ping()
	fmt.Println(pres, perr)

	// The params are wrapped in an array inside the Call function
	res, err := client.Call("system.info", []interface{}{})
	if err != nil {
		log.Fatalf("failed to call system.info: %v", err)
	}
	//log.Printf("Result: %s", res)
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, res, "", "\t")
	log.Printf("Result: %s", prettyJSON.String())

	// Graceful shutdown
	client.Close()
	log.Println("Client closed.")
}
