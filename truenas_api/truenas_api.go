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
	closeChan  chan bool
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
		url:        serverURL,
		conn:       conn,
		pending:    make(map[int]chan json.RawMessage),
		notifyChan: make(chan os.Signal, 1),
		closeChan:  make(chan bool),
	}

	go client.listen()

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

	if err := c.conn.WriteJSON(request); err != nil {
		return nil, fmt.Errorf("failed to send call: %w", err)
	}

	select {
	case res := <-responseChan:
		return res, nil
	case <-time.After(10 * time.Second):
		return nil, errors.New("call timed out")
	}
}

// listen handles incoming messages from the WebSocket server.
func (c *Client) listen() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("error reading message: %v", err)
			}
			c.Close()
			return
		}

		var response map[string]interface{}
		if err := json.Unmarshal(message, &response); err != nil {
			log.Printf("error unmarshaling response: %v", err)
			continue
		}

		// float64 "looks" wrong, but Javascript kinda only knows floats.
		if id, ok := response["id"].(float64); ok {
			callID := int(id)
			c.mu.Lock()
			if ch, exists := c.pending[callID]; exists {
				ch <- message
			}
			c.mu.Unlock()
		} else {
			log.Printf("received notification: %s", message)
		}
	}
}

// Ping sends a ping request to the server.
func (c *Client) Ping() error {
	_, err := c.Call("core.ping", []interface{}{}) // Pass an empty array as params
	return err
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
