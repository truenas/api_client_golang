package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/truenas/api_client_golang/truenas_api"
)

// CreateCertResult represents the result from certificate.create
type CreateCertResult struct {
	ID int `json:"id"`
}

func main() {
	// Test case demonstrating the job result issue fix
	addr := fmt.Sprintf("wss://%s/api/current", os.Getenv("TRUENAS_API_HOST"))
	testClient, err := truenas_api.NewClient(addr, false)
	if err != nil {
		log.Fatalf("creating truenas client: %s", err)
	}

	user := os.Getenv("TRUENAS_API_USER")
	pass := os.Getenv("TRUENAS_API_PASS")
	if err := testClient.Login(user, pass, ""); err != nil {
		log.Fatalf("logging in: %s", err)
	}

	if err := testClient.SubscribeToJobs(); err != nil {
		log.Fatalf("subscribing to jobs: %s", err)
	}

	name := "test-cert"

	certPath := os.Getenv("TRUENAS_API_CERT_PATH")
	if certPath == "" {
		// Use a dummy cert for testing if not provided
		certPath = "test.crt"
	}
	
	cert, err := os.ReadFile(certPath)
	if err != nil {
		log.Printf("Note: Could not read cert file, using dummy data: %s", err)
		cert = []byte("-----BEGIN CERTIFICATE-----\nDUMMY\n-----END CERTIFICATE-----")
	}

	keyPath := os.Getenv("TRUENAS_API_KEY_PATH")
	if keyPath == "" {
		keyPath = "test.key"
	}
	
	key, err := os.ReadFile(keyPath)
	if err != nil {
		log.Printf("Note: Could not read key file, using dummy data: %s", err)
		key = []byte("-----BEGIN PRIVATE KEY-----\nDUMMY\n-----END PRIVATE KEY-----")
	}

	log.Printf("creating cert: %q\n", name)
	params := map[string]interface{}{
		"name":        name,
		"certificate": string(cert),
		"privatekey":  string(key),
		"create_type": "CERTIFICATE_CREATE_IMPORTED",
	}

	cb := func(progress float64, state string, description string) {
		log.Printf("create job progress: %.2f%%, state: %s, description: %s\n", progress, state, description)
	}

	job, err := testClient.CallWithJob("certificate.create", []interface{}{params}, cb)
	if err != nil {
		log.Fatalf("error sending request: %s", err)
	}
	log.Printf("cert create job: %d\n", job.ID)

	for !job.Finished {
		select {
		case jobErr := <-job.DoneCh:
			if jobErr != "" {
				log.Fatalf("creating certificate: %s", jobErr)
			}
		case <-time.After(time.Second * 10):
			log.Fatal("timeout waiting for cert create job to finish")
		}
	}

	// The fix should allow job.Result to contain the actual map/struct
	log.Printf("cert create job finished, result type: %T\n", job.Result)
	log.Printf("cert create job result: %+v\n", job.Result)

	// Now we need to handle the result based on its actual type
	// It could be a map[string]interface{} directly
	if resultMap, ok := job.Result.(map[string]interface{}); ok {
		log.Printf("Result is a map, ID: %v\n", resultMap["id"])
		
		// Convert to JSON and back to struct if needed
		jsonBytes, err := json.Marshal(resultMap)
		if err != nil {
			log.Fatalf("marshaling result map: %s", err)
		}
		
		var result CreateCertResult
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			log.Fatalf("unmarshaling cert create result: %s", err)
		}
		
		log.Printf("cert created with ID: %d\n", result.ID)
	} else {
		log.Printf("Result is not a map, actual type: %T, value: %v\n", job.Result, job.Result)
	}
}