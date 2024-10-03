package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"truenas_api/truenas_api" // Replace with the correct package path
)

// parseArgs parses the JSON-style arguments passed via the command line.
func parseArgs(jsonArgs string) (interface{}, error) {
	// Parse the JSON arguments from the string passed via the command line
	var params interface{}
	err := json.Unmarshal([]byte(jsonArgs), &params)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON arguments: %w", err)
	}
	return params, nil
}

// printPrettyJSON prints the JSON data in a human-readable format.
func printPrettyJSON(data interface{}) {
	prettyResponse, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Failed to format JSON: %v", err)
	}
	fmt.Println(string(prettyResponse))
}

func main() {
	// Define command-line flags
	serverURL := flag.String("uri", "", "WebSocket server URI (e.g., ws://localhost:6000/websocket)")
	method := flag.String("method", "", "RPC method to call (e.g., core.ping)")
	jsonArgs := flag.String("params", "[]", "JSON-formatted arguments for the method (e.g., '[\"param1\", \"param2\"]')")
	timeout := flag.Int("timeout", 10, "Timeout in seconds for the call")
	verifySSL := flag.Bool("verifyssl", true, "Verify SSL certificates for wss:// connections")
	jobFlag := flag.Bool("job", false, "Use CallWithJob for methods that return a job ID")
	user := flag.String("U", "", "Username for login")
	pass := flag.String("P", "", "Password for login")
	apiKey := flag.String("api-key", "", "API key for login")

	// Parse the flags
	flag.Parse()

	// Validate input
	if *serverURL == "" || *method == "" {
		fmt.Println("Error: both --url and --method must be provided.")
		flag.Usage()
		os.Exit(1)
	}

	// Parse the JSON-style arguments
	params, err := parseArgs(*jsonArgs)
	if err != nil {
		log.Fatalf("Failed to parse JSON arguments: %v", err)
	}

	// Create a new WebSocket client
	client, err := truenas_api.NewClient(*serverURL, *verifySSL)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Attempt to log in if credentials or API key are provided
	if *apiKey != "" || (*user != "" && *pass != "") {
		err = client.Login(*user, *pass, *apiKey)
		if err != nil {
			log.Fatalf("Login failed: %v", err)
		}
		//fmt.Println("Successfully logged in.")
	}

	// Subscribe to job updates if using jobs
	if *jobFlag {
		err = client.SubscribeToJobs()
		if err != nil {
			log.Fatalf("Failed to subscribe to jobs: %v", err)
		}
	}

	if *jobFlag {
		// Use CallWithJob for methods that return a job ID
		//fmt.Printf("Calling method '%s' with job tracking...\n", *method)

		// Define the callback function to handle progress updates
		callback := func(progress float64, state string, desc string) {
			fmt.Printf("Job Progress: %.2f%%, State: %s, Description: %s\n", progress, state, desc)
		}

		// Call the method and get the Job object
		job, err := client.CallWithJob(*method, params, callback)
		if err != nil {
			log.Fatalf("CallWithJob failed: %v", err)
		}

		// Wait for the job to complete or timeout
		select {
		case errMsg := <-job.DoneCh:
			if errMsg != "" {
				log.Fatalf("Job failed: %s", errMsg)
			}
			// Fetch the final result of the job
			jobResult := job.Result
			fmt.Println("Job completed successfully. Result:")
			printPrettyJSON(jobResult)
		case <-time.After(time.Duration(*timeout) * time.Second):
			log.Fatalf("Job timed out after %d seconds", *timeout)
		}
	} else {
		// Use the regular Call method
		//fmt.Printf("Calling method '%s'...\n", *method)
		response, err := client.Call(*method, time.Duration(*timeout), params)
		if err != nil {
			log.Fatalf("RPC call failed: %v", err)
		}

		// Parse and pretty-print the response
		var parsedResponse interface{}
		err = json.Unmarshal(response, &parsedResponse)
		if err != nil {
			log.Fatalf("Failed to parse response: %v", err)
		}

		//fmt.Println("Response:")
		printPrettyJSON(parsedResponse)
	}
}
