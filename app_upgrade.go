package main

import (
	"log"
	"os"
	"truenas_api/truenas_api"
)

// example usage
func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: app_upgrade <server> <app_name>")
		os.Exit(1)
	}

	server := os.Args[1]
	app_name := os.Args[2]

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

	// Subscribe to job updates
	if err := client.SubscribeToJobs(); err != nil {
		log.Fatalf("failed to subscribe to job updates: %v", err)
	}

	job, err := client.CallWithJob("app.upgrade", []interface{}{app_name}, func(progress float64, state string, description string) {
		// This callback is called with the progress and state of the job
		log.Printf("Job Progress: %.2f%%, State: %s, Description: %s", progress, state, description)
	})
	if err != nil {
		log.Fatalf("failed to upgrade app: %v", err)
	}
	log.Printf("Started long-running job with ID: %d", job.ID)

	// Monitor the progress of the long-running job
	for !job.Finished {
		select {
		case progress := <-job.ProgressCh:
			log.Printf("Job progress: %.2f%%", progress)
		case err := <-job.DoneCh:
			if err != "" {
				log.Fatalf("Job failed: %v", err)
			} else {
				log.Println("Job completed successfully!")
			}
			client.Close()
		}
	}

	log.Println("Client closed.")
}
