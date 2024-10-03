package truenas_api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client encapsulates the connection to the WebSocket server.
type Client struct {
	url        string                       // WebSocket server URL
	conn       *websocket.Conn              // WebSocket connection instance
	mu         sync.Mutex                   // Mutex for ensuring thread-safety
	isClosed   bool                         // Indicates if the connection is closed
	callID     int                          // Unique ID for tracking each call
	pending    map[int]chan json.RawMessage // Stores pending calls, maps call IDs to response channels
	notifyChan chan os.Signal               // For handling notifications (e.g., OS signals)
	closeChan  chan struct{}                // Channel to signal when the connection should be closed
	jobs       *Jobs                        // Jobs manager to track long-running jobs
}

// Job represents a long-running job in TrueNAS.
type Job struct {
	ID         int64                                             // Job ID
	Method     string                                            // Method associated with the job
	State      string                                            // Current state of the job (e.g., "PENDING", "SUCCESS")
	Result     interface{}                                       // Result of the job once it finishes
	Progress   float64                                           // Progress of the job (0.0 to 100.0)
	Finished   bool                                              // Indicates if the job is finished
	ProgressCh chan float64                                      // Channel to report progress updates
	DoneCh     chan string                                       // Channel to signal when the job is done
	Callback   func(progress float64, state string, desc string) // Callback function to report progress and state
}

// Jobs manages long-running tasks.
type Jobs struct {
	client      *Client        // Reference to the WebSocket client
	jobs        map[int64]*Job // Maps job IDs to their corresponding job objects
	ownedJobIDs map[int64]bool // Stores the job IDs that were started by this client
	mu          sync.Mutex
}

// AddOwnedJob adds a job ID to the list of jobs started by this client.
func (j *Jobs) AddOwnedJob(jobID int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.ownedJobIDs[jobID] = true // Mark this job as "owned" by the client
}

// IsOwnedJob checks if a given job ID was started by this client.
func (j *Jobs) IsOwnedJob(jobID int64) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, exists := j.ownedJobIDs[jobID] // Check if the job ID exists in ownedJobIDs
	return exists
}

// NewJobs creates a new Jobs manager.
func NewJobs(client *Client) *Jobs {
	return &Jobs{
		client:      client,
		jobs:        make(map[int64]*Job),
		ownedJobIDs: make(map[int64]bool),
	}
}

// AddJob adds a new job to the Jobs manager.
func (j *Jobs) AddJob(jobID int64, method string) *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	job := &Job{
		ID:         jobID,
		Method:     method,
		State:      "PENDING",
		ProgressCh: make(chan float64),
		DoneCh:     make(chan string),
	}
	j.jobs[jobID] = job // Add job to jobs map
	return job
}

// GetJob retrieves a job by its ID.
func (j *Jobs) GetJob(jobID int64) (*Job, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, exists := j.jobs[jobID] // Retrieve the job by ID
	return job, exists
}

// RemoveJob removes a completed job from the Jobs manager.
func (j *Jobs) RemoveJob(jobID int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.jobs, jobID) // Remove job from jobs map
}

// UpdateJobState updates the state of a long-running job.
func (j *Jobs) UpdateJobState(jobID int64, state string, progress float64, result interface{}, err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, exists := j.jobs[jobID]
	if !exists {
		return // If the job doesn't exist, return
	}
	job.State = state
	job.Progress = progress
	if state == "SUCCESS" || state == "FAILED" {
		job.Finished = true
		job.Result = result
		job.DoneCh <- err     // Send error (if any) to the done channel
		close(job.ProgressCh) // Close progress channel after job completion
		close(job.DoneCh)     // Close done channel after job completion
	}
}

func (c *Client) SubscribeToJobs() error {
	params := []interface{}{"core.get_jobs"} // Core function to subscribe to job updates

	// Make the subscription call via WebSocket
	res, err := c.Call("core.subscribe", 10, params)
	if err != nil {
		return err
	}

	// Parse subscription result
	var response map[string]interface{}
	if err := json.Unmarshal(res, &response); err != nil {
		return fmt.Errorf("failed to parse subscription response: %w", err)
	}

	return nil
}

// NewClient creates a new WebSocket client connection.
func NewClient(serverURL string, verifySSL bool) (*Client, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Configure WebSocket connection options
	dialer := websocket.DefaultDialer
	if u.Scheme == "wss" && !verifySSL {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // Disable SSL verification for wss
	}

	// Establish the WebSocket connection
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	client := &Client{
		url:       serverURL,
		conn:      conn,
		pending:   make(map[int]chan json.RawMessage),
		closeChan: make(chan struct{}),
		jobs:      NewJobs(nil),
	}

	client.jobs = NewJobs(client)

	go client.listen() // Start listening for WebSocket messages

	return client, nil
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return nil // Return if connection is already closed
	}
	c.isClosed = true
	close(c.closeChan) // Signal that the connection is closed
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}
	return c.conn.Close() // Close the actual WebSocket connection
}

// Call sends an RPC call to the server and waits for a response.
func (c *Client) Call(method string, timeout time.Duration, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.callID++ // Increment callID for each call
	callID := c.callID
	responseChan := make(chan json.RawMessage, 1) // Create channel to receive the response
	c.pending[callID] = responseChan              // Store the callID and response channel
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, callID) // Clean up the pending map after response is received
		c.mu.Unlock()
	}()

	// Create the RPC request payload
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      callID,
		"params":  params,
	}

	// Send the request to the WebSocket server
	if err := c.conn.WriteJSON(request); err != nil {
		return nil, fmt.Errorf("failed to send call: %w", err)
	}

	// Wait for the response or timeout
	select {
	case res := <-responseChan:
		return res, nil
	case <-time.After(timeout * time.Second):
		return nil, errors.New("call timed out")
	}
}

// listen listens for incoming WebSocket messages.
func (c *Client) listen() {
	for {
		select {
		case <-c.closeChan: // If the connection is closed, stop listening
			return
		default:
			_, message, err := c.conn.ReadMessage() // Read message from WebSocket server
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					// log.Printf("error reading message: %v", err) // Log any non-close error
				}
				c.Close()
				return
			}

			var response map[string]interface{}
			if err := json.Unmarshal(message, &response); err != nil {
				continue // Ignore if message can't be parsed
			}

			// Handle collection update (e.g., job progress updates)
			if method, ok := response["method"].(string); ok && method == "collection_update" {
				params := response["params"].(map[string]interface{})
				jobID := int64(params["id"].(float64))
				fields := params["fields"].(map[string]interface{})

				// Only handle jobs started by this client
				if c.jobs.IsOwnedJob(jobID) {
					progress := fields["progress"].(map[string]interface{})
					description, _ := progress["description"].(string)
					percent, _ := progress["percent"].(float64)
					state, _ := fields["state"].(string)
					result, _ := fields["result"].(string)
					errors, _ := fields["error"].(string)

					// Update the job state in the Jobs manager
					c.jobs.UpdateJobState(jobID, state, percent, result, errors)

					// Trigger the callback if it exists
					if job, exists := c.jobs.jobs[jobID]; exists && job.Callback != nil {
						job.Callback(percent, state, description)
					}
				}
				continue
			}

			// Handle RPC responses by matching call ID
			if id, ok := response["id"].(float64); ok {
				callID := int(id)
				c.mu.Lock()
				if ch, exists := c.pending[callID]; exists {
					ch <- message // Send message to pending call's channel
				}
				c.mu.Unlock()
			}
		}
	}
}

// CallWithJob sends an RPC call that returns a job ID and tracks the long-running job.
func (c *Client) CallWithJob(method string, params interface{}, callback func(progress float64, state string, desc string)) (*Job, error) {
	// Call the API method
	res, err := c.Call(method, 10, params)
	if err != nil {
		return nil, err
	}

	// Parse the response and extract the job ID
	var response map[string]interface{}
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if errorData, exists := response["error"]; exists {
		return nil, fmt.Errorf("API error: %v", errorData)
	}

	// Extract job ID from the response
	jobID, ok := response["result"].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected response format for job")
	}

	// Add the job to the Jobs manager
	job := c.jobs.AddJob(int64(jobID), method)

	// Mark this job as owned by this client
	c.jobs.AddOwnedJob(int64(jobID))

	// Set the callback function for job updates
	job.Callback = callback

	// Return the Job instance to allow tracking
	return job, nil
}

// Ping sends a ping request to the server to check connectivity.
func (c *Client) Ping() (string, error) {
	res, err := c.Call("core.ping", 10, []interface{}{}) // Empty array as params

	if err != nil {
		return "", err
	}

	// Parse the result from the response
	var response map[string]interface{}
	if err := json.Unmarshal(res, &response); err != nil {
		return "", fmt.Errorf("failed to parse ping response: %w", err)
	}

	// Return the result (e.g., "pong") from the response
	if result, exists := response["result"].(string); exists {
		return result, nil
	}

	return "", errors.New("unexpected ping response format")
}

// Login attempts to log in using either username/password or an API key.
func (c *Client) Login(username, password, apiKey string) error {
	var params interface{}
	var method string

	if apiKey != "" {
		// Use API key login
		method = "auth.login_with_api_key"
		params = []interface{}{apiKey}
	} else if username != "" && password != "" {
		// Use username and password login
		method = "auth.login"
		params = []interface{}{username, password}
	} else {
		return errors.New("either username/password or API key must be provided")
	}

	// Make the login call
	res, err := c.Call(method, 10, params)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(res, &response); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	// Check if there's an error in the login response
	if errorData, exists := response["error"]; exists {
		return fmt.Errorf("login error: %v", errorData)
	}

	// Return success if login result is true
	if result, exists := response["result"]; exists && result == true {
		return nil
	}

	return errors.New("login failed, unexpected response")
}
