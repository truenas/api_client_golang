package main

import (
	"log"
	"os"
	"truenas_api/truenas_api"
)

const (
	apiKeyEnv       = "TRUENAS_API_KEY"
	defaultProtocol = "ws://"
	apiPath         = "/api/current"
)

// checkEnvOrExit checks if the environment variable is set and returns its value or exits if not set.
func checkEnvOrExit(envVar string) string {
	value, exists := os.LookupEnv(envVar)
	if !exists {
		log.Fatalf("Environment variable %s not set", envVar)
		os.Exit(1)
	}
	return value
}

// logFatalAndExit logs an error message and exits the application.
func logFatalAndExit(format string, v ...interface{}) {
	log.Fatalf(format, v...)
	os.Exit(1)
}

// connectClient initializes and returns a TrueNAS API client.
func connectClient(server string) *truenas_api.Client {
	serverURL := defaultProtocol + server + apiPath
	client, err := truenas_api.NewClient(serverURL, false)
	if err != nil {
		logFatalAndExit("Failed to connect: %v", err)
	}
	return client
}

// loginClient logs into the TrueNAS API client.
func loginClient(client *truenas_api.Client) {
	username, password := "", ""
	apiKey := checkEnvOrExit(apiKeyEnv)

	err := client.Login(username, password, apiKey)
	if err != nil {
		logFatalAndExit("Login failed: %v", err)
	}
	log.Println("Login successful!")
}

// main is the entry point of the application.
func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: app_upgrade <server> <app_name>")
		os.Exit(1)
	}

	server, appName := os.Args[1], os.Args[2]
	log.Printf("Connecting to TrueNAS server at %s", server)

	client := connectClient(server)
	defer client.Close()

	loginClient(client)

	// Subscribe to job updates
	if err := client.SubscribeToJobs(); err != nil {
		logFatalAndExit("Failed to subscribe to job updates: %v", err)
	}

	// Define the callback to handle job progress updates.
	job, err := client.CallWithJob("app.upgrade", []interface{}{appName}, func(progress float64, state string, description string) {
		log.Printf("Job Progress: %.2f%%, State: %s, Description: %s", progress, state, description)
	})
	if err != nil {
		logFatalAndExit("Failed to upgrade app: %v", err)
	}
	log.Printf("Started long-running job with ID: %d", job.ID)

	// Monitor the progress of the job.
	for !job.Finished {
		select {
		case progress := <-job.ProgressCh:
			log.Printf("Job progress: %.2f%%", progress)
		case err := <-job.DoneCh:
			if err != "" {
				logFatalAndExit("Job failed: %v", err)
			} else {
				log.Println("Job completed successfully!")
			}
			client.Close()
		}
	}

	log.Println("Client closed.")
}
