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

	// Example call to create a user
	params := map[string]interface{}{
		"full_name":    "John Doe",
		"username":     "user2",
		"password":     "pass",
		"group_create": true,
		//"job":          true,
	}

	job, err := client.CallWithJob("user.create", []interface{}{params})
	if err != nil {
		log.Fatalf("failed to create user: %v", err)
	}
	log.Printf("Started long-running job with ID: %d", job.ID)

	// Subscribe to job updates
	if err := client.SubscribeToJobs(); err != nil {
		log.Fatalf("failed to subscribe to job updates: %v", err)
	}

	// Monitor the progress of the long-running job
	for !job.Finished {
		select {
		case progress := <-job.ProgressCh:
			log.Printf("Job progress: %.2f%%", progress)
		case err := <-job.DoneCh:
			if err != nil {
				log.Fatalf("Job failed: %v", err)
			}
			log.Println("Job completed successfully!")
		}
	}

	// Close the connection after the work is done
	client.Close()
	log.Println("Client closed.")
}