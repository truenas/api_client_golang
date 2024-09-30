package truenas_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client encapsulates the connection to the WebSocket server.
type Client struct {
	url        string
	conn       *websocket.Conn
	mu         sync.Mutex
	isClosed   bool
	callID     int
	pending    map[int]chan json.RawMessage
	notifyChan chan os.Signal
	closeChan  chan struct{}
	jobs       *Jobs // Jobs manager to track long-running jobs
}

// Job represents a long-running job in TrueNAS.
type Job struct {
	ID         int64
	Method     string
	State      string
	Result     interface{}
	Progress   float64
	Finished   bool
	ProgressCh chan float64
	DoneCh     chan error
}

// Jobs manages long-running tasks.
type Jobs struct {
	client      *Client
	jobs        map[int64]*Job
	ownedJobIDs map[int64]bool // Store the job IDs that were started by this client
	mu          sync.Mutex
}

// AddOwnedJob adds a job ID to the list of jobs started by this client.
func (j *Jobs) AddOwnedJob(jobID int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.ownedJobIDs[jobID] = true
}

// IsOwnedJob checks if a given job ID was started by this client.
func (j *Jobs) IsOwnedJob(jobID int64) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, exists := j.ownedJobIDs[jobID]
	return exists
}

// NewJobs creates a new Jobs manager.
func NewJobs(client *Client) *Jobs {
	return &Jobs{
		client:      client,
		jobs:        make(map[int64]*Job),
		ownedJobIDs: map[int64]bool{},
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
		DoneCh:     make(chan error),
	}
	j.jobs[jobID] = job
	return job
}

// GetJob retrieves a job by its ID.
func (j *Jobs) GetJob(jobID int64) (*Job, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, exists := j.jobs[jobID]
	return job, exists
}

// RemoveJob removes a completed job from the Jobs manager.
func (j *Jobs) RemoveJob(jobID int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.jobs, jobID)
}

// UpdateJobState updates the state of a long-running job.
func (j *Jobs) UpdateJobState(jobID int64, state string, progress float64, result interface{}, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, exists := j.jobs[jobID]
	if !exists {
		return
	}
	job.State = state
	job.Progress = progress
	if state == "SUCCESS" || state == "FAILED" {
		job.Finished = true
		job.Result = result
		job.DoneCh <- err
		close(job.ProgressCh)
		close(job.DoneCh)
	}
}

// SubscribeToJobs subscribes to job updates from the WebSocket.
func (c *Client) SubscribeToJobs() error {
	// params := []interface{}{"core.get_jobs"}
	// res, err := c.Call("core.subscribe", params)
	//params := []interface{}
	//res, err := c.Call("core.subscribe", []interface{}{jobIDs})
	//if err != nil {
	//	return err
	//}

	params := []interface{}{"core.get_jobs"}

	// Make the subscription call
	res, err := c.Call("core.subscribe", params)
	if err != nil {
		return err
	}

	// Parse subscription result
	var response map[string]interface{}
	if err := json.Unmarshal(res, &response); err != nil {
		return fmt.Errorf("failed to parse subscription response: %w", err)
	}

	log.Println("Subscribed to job updates successfully!")
	return nil
}

// NewClient creates a new WebSocket client.
func NewClient(serverURL string) (*Client, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
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

	go client.listen() // Start listening for incoming messages

	return client, nil
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return nil
	}
	c.isClosed = true
	close(c.closeChan)
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}
	return c.conn.Close()
}

// Call sends an RPC call to the server and waits for a response.
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.callID++
	callID := c.callID
	responseChan := make(chan json.RawMessage, 1)
	c.pending[callID] = responseChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, callID)
		c.mu.Unlock()
	}()

	// For user.create and similar calls, we need to wrap params in an array
	// For auth.login, we will handle it separately
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      callID,
		"params":  params,
	}

	// Log that we are sending the request
	log.Printf("Sending request: %v", request)

	if err := c.conn.WriteJSON(request); err != nil {
		return nil, fmt.Errorf("failed to send call: %w", err)
	}

	select {
	case res := <-responseChan:
		//log.Printf("Received response: %v", string(res))
		return res, nil
	case <-time.After(300 * time.Second):
		return nil, errors.New("call timed out")
	}
}

// listen handles incoming messages from the WebSocket server.
func (c *Client) listen() {
	for {
		select {
		case <-c.closeChan:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("error reading message: %v", err)
				}
				c.Close()
				return
			}

			// Log the message received
			//log.Printf("Received message: %s", message)

			var response map[string]interface{}
			if err := json.Unmarshal(message, &response); err != nil {
				log.Printf("error unmarshaling response: %v", err)
				continue
			}

			// Handle job progress updates
			//log.Printf("method: %v", response["method"])
			if method, ok := response["method"].(string); ok && method == "collection_update" {
				//log.Printf("Message is a job progress update: %s", message)
				params := response["params"].(map[string]interface{})
				jobID := int64(params["id"].(float64))
				fields := params["fields"].(map[string]interface{})

				// Only handle jobs started by this client
				if c.jobs.IsOwnedJob(jobID) {
					progress, ok := fields["progress"].(map[string]interface{})
					description, ok := progress["description"].(string)
					percent, ok := progress["percent"].(float64)
					if !ok {
						percent = 0
					}
					state, ok := fields["state"].(string)
					if !ok {
						state = "unknown"
					}
					// Log job updates
					log.Printf("Job update (started by this client): ID=%d, progress=%.2f%%, description = %s, state=%s", jobID, percent, description, state)

					// Update the job state in the Jobs manager
					c.jobs.UpdateJobState(jobID, state, percent, nil, nil)
				}
				continue
			}

			// float64 "looks" wrong, but Javascript kinda only knows floats.
			if id, ok := response["id"].(float64); ok {
				callID := int(id)
				c.mu.Lock()
				if ch, exists := c.pending[callID]; exists {
					ch <- message
				}
				//log.Printf("received response: %s", message)
				c.mu.Unlock()
			} else {
				// log.Printf("received notification: %s", message)
			}
		}
	}
}

// CallWithJob sends an RPC call that returns a job ID and tracks the long-running job.
func (c *Client) CallWithJob(method string, params interface{}) (*Job, error) {
	// Call the API method
	res, err := c.Call(method, params)
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

	jobID, ok := response["result"].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected response format for job")
	}

	// Add the job to the Jobs manager
	job := c.jobs.AddJob(int64(jobID), method)

	// Mark this job as owned by this client
	c.jobs.AddOwnedJob(int64(jobID))

	// Return the Job instance to allow tracking
	return job, nil
}

// Ping sends a ping request to the server.
func (c *Client) Ping() (string, error) {
	res, err := c.Call("core.ping", []interface{}{}) // Pass an empty array as params

	if err != nil {
		return "", err
	}

	// Parse the result from the response
	var response map[string]interface{}
	if err := json.Unmarshal(res, &response); err != nil {
		return "", fmt.Errorf("failed to parse ping response: %w", err)
	}

	// Check if there's an error in the ping response
	if errorData, exists := response["error"]; exists {
		return "", fmt.Errorf("ping error: %v", errorData)
	}

	// Return the result (e.g., "pong") from the response
	if result, exists := response["result"].(string); exists {
		return result, nil
	}

	return "", errors.New("unexpected ping response format")
}

// Login attempts to log in using username/password or API key.
// If username and password are provided, they are used for login. Otherwise, it will try to use the API key.
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
		params = []interface{}{username, password} // Note: params are passed as-is, no array wrapping here
	} else {
		return errors.New("either username/password or API key must be provided")
	}

	// Make the login call
	res, err := c.Call(method, params)
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

	// Check the result, depending on the success of the login
	if result, exists := response["result"]; exists {
		if result == true {
			return nil
		}
	}

	return errors.New("login failed, unexpected response")
}
