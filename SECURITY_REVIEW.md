# Security Review Report - TrueNAS Go API Client

**Review Date:** 2025-10-21
**Reviewed By:** Claude (Security Analysis)
**Codebase:** github.com/truenas/api_client_golang

## Executive Summary

This security review identified **18 security issues** across various severity levels in the TrueNAS Go WebSocket API Client. The most critical findings involve unsafe type assertions that could cause runtime panics, potential race conditions in concurrent code, and weak input validation that could lead to injection vulnerabilities.

**Severity Distribution:**
- 🔴 **Critical:** 3 issues
- 🟠 **High:** 5 issues
- 🟡 **Medium:** 7 issues
- 🟢 **Low:** 3 issues

---

## Critical Issues

### 1. 🔴 Unsafe Type Assertions Without Error Checking

**File:** `truenas_api/truenas_api.go`
**Lines:** 300-302, 316-321
**Severity:** Critical
**CWE:** CWE-248 (Uncaught Exception)

**Description:**
Multiple type assertions in the message handling code lack proper error checking, which will cause the application to panic if the WebSocket server sends unexpected data formats.

**Vulnerable Code:**
```go
// Line 300-302
params := response["params"].(map[string]interface{})
jobID := int64(params["id"].(float64))
fields := params["fields"].(map[string]interface{})

// Line 316-321
progress := fields["progress"].(map[string]interface{})
description, _ := progress["description"].(string)
percent, _ := progress["percent"].(float64)
state, _ := fields["state"].(string)
```

**Impact:**
- Application crashes when receiving malformed messages
- Denial of Service (DoS) vulnerability
- No graceful error handling for protocol violations

**Recommendation:**
```go
// Safe type assertion pattern
params, ok := response["params"].(map[string]interface{})
if !ok {
    log.Printf("Invalid params format in collection_update")
    continue
}

jobIDFloat, ok := params["id"].(float64)
if !ok {
    log.Printf("Invalid job ID format")
    continue
}
jobID := int64(jobIDFloat)
```

---

### 2. 🔴 Race Condition in Job Callback Access

**File:** `truenas_api/truenas_api.go`
**Lines:** 327-329
**Severity:** Critical
**CWE:** CWE-362 (Concurrent Execution using Shared Resource with Improper Synchronization)

**Description:**
The code accesses `job.Callback` without mutex protection in the `listen()` goroutine, while the Job struct can be modified from other goroutines.

**Vulnerable Code:**
```go
// Line 327-329
if job, exists := c.jobs.jobs[jobID]; exists && job.Callback != nil {
    job.Callback(percent, state, description)
}
```

**Impact:**
- Data races when accessing job callbacks
- Potential crashes due to concurrent map access
- Unpredictable behavior in multi-threaded environments

**Recommendation:**
```go
// Add mutex protection to Job struct
type Job struct {
    mu         sync.RWMutex
    ID         int64
    Callback   func(progress float64, state string, desc string)
    // ... other fields
}

// In listen()
c.jobs.mu.Lock()
job, exists := c.jobs.jobs[jobID]
c.jobs.mu.Unlock()

if exists && job.Callback != nil {
    job.mu.RLock()
    callback := job.Callback
    job.mu.RUnlock()

    if callback != nil {
        callback(percent, state, description)
    }
}
```

---

### 3. 🔴 TLS Certificate Verification Can Be Disabled

**File:** `truenas_api/truenas_api.go`
**Line:** 159
**Severity:** Critical
**CWE:** CWE-295 (Improper Certificate Validation)

**Description:**
The client allows disabling TLS certificate verification via the `verifySSL` parameter, which enables man-in-the-middle (MITM) attacks.

**Vulnerable Code:**
```go
// Line 158-160
if u.Scheme == "wss" && !verifySSL {
    dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}
```

**Impact:**
- Man-in-the-middle attacks on WebSocket connections
- Credentials and API keys transmitted to attackers
- Complete compromise of session confidentiality

**Recommendation:**
1. Remove the option entirely for production use
2. Add prominent warnings in documentation
3. Implement certificate pinning for high-security deployments
4. Add environment variable check to prevent accidental production use:

```go
if u.Scheme == "wss" && !verifySSL {
    if os.Getenv("TRUENAS_ALLOW_INSECURE") != "true" {
        return nil, errors.New("TLS verification disabled without TRUENAS_ALLOW_INSECURE=true")
    }
    log.Println("WARNING: TLS certificate verification is disabled")
    dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}
```

---

## High Severity Issues

### 4. 🟠 Credentials Exposed in Command-Line Arguments

**File:** `truenas_go.go`
**Lines:** 42-44, 71
**Severity:** High
**CWE:** CWE-214 (Invocation of Process Using Visible Sensitive Information)

**Description:**
Username, password, and API keys are accepted as command-line flags, which are visible in process listings (`ps`, `top`, `/proc` filesystem).

**Vulnerable Code:**
```go
pass := flag.String("P", "", "Password for login")
apiKey := flag.String("api-key", "", "API key for login")
```

**Impact:**
- Credentials visible to all users on the system via `ps aux`
- Credentials logged in shell history
- Credentials exposed in process monitoring tools

**Recommendation:**
```go
// 1. Use environment variables only (already supported in examples)
// 2. Add warning when flags are used
if *pass != "" || *apiKey != "" {
    log.Println("WARNING: Passing credentials via command-line flags is insecure")
    log.Println("Use environment variables TRUENAS_PASSWORD or TRUENAS_API_KEY instead")
}

// 3. Read from stdin or secure prompt
if *pass == "" && os.Getenv("TRUENAS_PASSWORD") == "" {
    fmt.Print("Enter password: ")
    passBytes, _ := term.ReadPassword(int(os.Stdin.Fd()))
    password = string(passBytes)
}
```

---

### 5. 🟠 No Input Validation on Method Names

**File:** `truenas_api/truenas_api.go`
**Lines:** 238-275
**Severity:** High
**CWE:** CWE-20 (Improper Input Validation)

**Description:**
The `Call()` function does not validate method names before sending them to the server, potentially allowing injection attacks or abuse.

**Vulnerable Code:**
```go
// Line 256-261
request := map[string]interface{}{
    "jsonrpc": "2.0",
    "method":  method,  // No validation
    "id":      callID,
    "params":  params,
}
```

**Impact:**
- Potential command injection if server-side parsing is vulnerable
- Ability to call internal/undocumented methods
- No rate limiting or access control at client level

**Recommendation:**
```go
// Add method name validation
func validateMethodName(method string) error {
    if method == "" {
        return errors.New("method name cannot be empty")
    }

    // Validate format: namespace.method
    if !regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*$`).MatchString(method) {
        return fmt.Errorf("invalid method name format: %s", method)
    }

    // Optional: whitelist of allowed methods
    return nil
}

// In Call()
if err := validateMethodName(method); err != nil {
    return nil, err
}
```

---

### 6. 🟠 Library Code Uses log.Fatalf Instead of Returning Errors

**File:** `truenas_api/truenas_api.go`
**Line:** 204
**Severity:** High
**CWE:** CWE-703 (Improper Check or Handling of Exceptional Conditions)

**Description:**
The `NewClientFromConn()` function uses `log.Fatalf()` which terminates the entire application, preventing proper error handling by calling code.

**Vulnerable Code:**
```go
// Line 202-205
wsConn, _, err := dialer.Dial(wsURL.String(), nil)
if err != nil {
    log.Fatalf("Failed to connect to WebSocket: %v", err)
}
```

**Impact:**
- Library code cannot be used in production services
- No opportunity for graceful degradation
- Crashes entire application on connection failure

**Recommendation:**
```go
wsConn, _, err := dialer.Dial(wsURL.String(), nil)
if err != nil {
    return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
}
```

---

### 7. 🟠 No Timeout on WebSocket Read Operations

**File:** `truenas_api/truenas_api.go`
**Lines:** 278-344
**Severity:** High
**CWE:** CWE-400 (Uncontrolled Resource Consumption)

**Description:**
The `listen()` function performs blocking reads without timeouts, potentially causing goroutine leaks.

**Vulnerable Code:**
```go
// Line 284
_, message, err := c.conn.ReadMessage()
```

**Impact:**
- Goroutine hangs indefinitely if connection stalls
- Resource exhaustion in long-running applications
- No mechanism to detect dead connections

**Recommendation:**
```go
// Set read deadline
func (c *Client) listen() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-c.closeChan:
            return
        case <-ticker.C:
            // Set read deadline for next read
            c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
        default:
            c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
            _, message, err := c.conn.ReadMessage()
            // ... rest of logic
        }
    }
}
```

---

### 8. 🟠 JSON Parameter Injection Risk

**File:** `truenas_go.go`
**Lines:** 14-23
**Severity:** High
**CWE:** CWE-502 (Deserialization of Untrusted Data)

**Description:**
The `parseArgs()` function accepts arbitrary JSON without validation, potentially allowing injection of malicious data structures.

**Vulnerable Code:**
```go
// Line 15-23
func parseArgs(jsonArgs string) (interface{}, error) {
    var params interface{}
    err := json.Unmarshal([]byte(jsonArgs), &params)
    if err != nil {
        return nil, fmt.Errorf("invalid JSON arguments: %w", err)
    }
    return params, nil
}
```

**Impact:**
- Arbitrary JSON structures sent to API
- Potential for nested/deep objects causing DoS
- No schema validation before sending

**Recommendation:**
```go
func parseArgs(jsonArgs string) (interface{}, error) {
    var params interface{}

    // Limit JSON size
    if len(jsonArgs) > 1024*1024 { // 1MB limit
        return nil, errors.New("JSON arguments too large")
    }

    decoder := json.NewDecoder(strings.NewReader(jsonArgs))
    decoder.DisallowUnknownFields() // Stricter parsing

    if err := decoder.Decode(&params); err != nil {
        return nil, fmt.Errorf("invalid JSON arguments: %w", err)
    }

    // Validate structure depth
    if depth := getJSONDepth(params); depth > 10 {
        return nil, errors.New("JSON nesting too deep")
    }

    return params, nil
}
```

---

## Medium Severity Issues

### 9. 🟡 Error Logging Commented Out

**File:** `truenas_api/truenas_api.go`
**Line:** 287
**Severity:** Medium
**CWE:** CWE-778 (Insufficient Logging)

**Description:**
Error logging in the message reading loop is commented out, hiding connection issues.

**Vulnerable Code:**
```go
// Line 286-288
if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
    // log.Printf("error reading message: %v", err)
}
```

**Recommendation:**
```go
if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
    log.Printf("websocket read error: %v", err)
}
```

---

### 10. 🟡 No Rate Limiting on API Calls

**File:** `truenas_api/truenas_api.go`
**Severity:** Medium
**CWE:** CWE-770 (Allocation of Resources Without Limits or Throttling)

**Description:**
No protection against excessive API calls that could overwhelm the server or trigger rate limiting.

**Recommendation:**
```go
import "golang.org/x/time/rate"

type Client struct {
    // ... existing fields
    limiter *rate.Limiter
}

func NewClient(serverURL string, verifySSL bool) (*Client, error) {
    client := &Client{
        // ... existing initialization
        limiter: rate.NewLimiter(rate.Limit(10), 20), // 10 req/sec, burst of 20
    }
    return client, nil
}

func (c *Client) Call(method string, timeoutSeconds int64, params interface{}) (json.RawMessage, error) {
    if err := c.limiter.Wait(context.Background()); err != nil {
        return nil, fmt.Errorf("rate limit error: %w", err)
    }
    // ... rest of function
}
```

---

### 11. 🟡 Potential Memory Leak in Pending Calls Map

**File:** `truenas_api/truenas_api.go`
**Lines:** 246, 251-253
**Severity:** Medium
**CWE:** CWE-401 (Missing Release of Memory after Effective Lifetime)

**Description:**
If a call times out or the connection drops before response, the pending map entry might not be cleaned up properly.

**Vulnerable Code:**
```go
c.pending[callID] = responseChan

defer func() {
    c.mu.Lock()
    delete(c.pending, callID)
    c.mu.Unlock()
}()
```

**Recommendation:**
```go
// Add periodic cleanup
func (c *Client) cleanupStaleRequests() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            c.mu.Lock()
            // Clean up requests older than 2 hours
            for id, ch := range c.pending {
                select {
                case <-ch:
                    delete(c.pending, id)
                default:
                }
            }
            c.mu.Unlock()
        case <-c.closeChan:
            return
        }
    }
}
```

---

### 12. 🟡 No Panic Recovery in Goroutines

**File:** `truenas_api/truenas_api.go`
**Lines:** 179, 278
**Severity:** Medium
**CWE:** CWE-248 (Uncaught Exception)

**Description:**
The `listen()` goroutine has no panic recovery, which would crash the entire application.

**Recommendation:**
```go
func (c *Client) listen() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("panic in listen goroutine: %v", r)
            c.Close()
        }
    }()

    // ... rest of function
}
```

---

### 13. 🟡 Job Channels May Be Written After Close

**File:** `truenas_api/truenas_api.go`
**Lines:** 121-123
**Severity:** Medium
**CWE:** CWE-366 (Race Condition within a Thread)

**Description:**
Job state updates might attempt to write to already-closed channels if multiple updates arrive simultaneously.

**Vulnerable Code:**
```go
job.DoneCh <- err
close(job.ProgressCh)
close(job.DoneCh)
```

**Recommendation:**
```go
type Job struct {
    // ... existing fields
    once sync.Once
}

func (j *Jobs) UpdateJobState(jobID int64, state string, progress float64, result interface{}, err string) {
    // ... existing code

    if state == "SUCCESS" || state == "FAILED" {
        job.Finished = true
        job.Result = result

        job.once.Do(func() {
            select {
            case job.DoneCh <- err:
            default:
            }
            close(job.ProgressCh)
            close(job.DoneCh)
        })
    }
}
```

---

### 14. 🟡 No Maximum Response Size Limit

**File:** `truenas_api/truenas_api.go`
**Line:** 284
**Severity:** Medium
**CWE:** CWE-770 (Allocation of Resources Without Limits)

**Description:**
The WebSocket client does not limit the size of incoming messages, potentially causing memory exhaustion.

**Recommendation:**
```go
func NewClient(serverURL string, verifySSL bool) (*Client, error) {
    // ... existing code

    conn.SetReadLimit(10 * 1024 * 1024) // 10MB max message size

    // ... rest of function
}
```

---

### 15. 🟡 No Certificate Pinning

**File:** `truenas_api/truenas_api.go`
**Lines:** 157-160
**Severity:** Medium
**CWE:** CWE-295 (Improper Certificate Validation)

**Description:**
The client trusts any valid certificate from a CA, without option to pin specific certificates.

**Recommendation:**
```go
func NewClientWithCertPin(serverURL string, certPin string) (*Client, error) {
    u, err := url.Parse(serverURL)
    if err != nil {
        return nil, fmt.Errorf("invalid URL: %w", err)
    }

    dialer := websocket.DefaultDialer
    if u.Scheme == "wss" {
        dialer.TLSClientConfig = &tls.Config{
            VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
                if certPin == "" {
                    return nil
                }

                for _, cert := range rawCerts {
                    hash := sha256.Sum256(cert)
                    if hex.EncodeToString(hash[:]) == certPin {
                        return nil
                    }
                }
                return errors.New("certificate pin mismatch")
            },
        }
    }
    // ... rest of function
}
```

---

## Low Severity Issues

### 16. 🟢 Hardcoded Password in Example Code

**File:** `examples/user_add/user_add.go`
**Line:** 44
**Severity:** Low
**CWE:** CWE-798 (Use of Hard-coded Credentials)

**Description:**
Example code contains hardcoded password "pass" for demonstration purposes.

**Vulnerable Code:**
```go
params := map[string]interface{}{
    "password": "pass",
}
```

**Recommendation:**
```go
// Add comment warning
params := map[string]interface{}{
    "password": "pass", // WARNING: Use strong passwords in production
}
```

---

### 17. 🟢 Dummy Certificate/Key in Test Code

**File:** `examples/test_job_result/main.go`
**Lines:** 47, 58
**Severity:** Low
**CWE:** CWE-798 (Use of Hard-coded Credentials)

**Description:**
Test code includes dummy certificate and private key as fallback.

**Recommendation:**
Add clear warnings that these are test-only values.

---

### 18. 🟢 No Request ID Overflow Protection

**File:** `truenas_api/truenas_api.go`
**Line:** 243
**Severity:** Low
**CWE:** CWE-190 (Integer Overflow)

**Description:**
The `callID` counter will overflow after 2^31 calls (on 32-bit) or 2^63 calls (on 64-bit).

**Recommendation:**
```go
c.callID++
if c.callID <= 0 {
    c.callID = 1 // Reset on overflow
}
```

---

## Additional Security Recommendations

### 1. Add Security Headers
Consider implementing WebSocket subprotocol negotiation for version compatibility.

### 2. Implement Audit Logging
Add security-relevant event logging:
- Authentication attempts (success/failure)
- API calls made
- Connection state changes
- Error conditions

### 3. Add Context Support
Update API to use `context.Context` for cancellation and timeouts:
```go
func (c *Client) CallWithContext(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
```

### 4. Connection Health Checks
Implement periodic ping/pong to detect dead connections:
```go
func (c *Client) healthCheck() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if _, err := c.Ping(); err != nil {
                log.Printf("health check failed: %v", err)
                c.Close()
                return
            }
        case <-c.closeChan:
            return
        }
    }
}
```

### 5. Secure Defaults
- Enable TLS verification by default
- Require environment variable to disable
- Add warnings for insecure configurations

---

## Compliance Considerations

### OWASP Top 10 (2021)
- **A02:2021 - Cryptographic Failures**: Issues #3 (InsecureSkipVerify)
- **A03:2021 - Injection**: Issues #5, #8 (No input validation)
- **A04:2021 - Insecure Design**: Issue #10 (No rate limiting)
- **A05:2021 - Security Misconfiguration**: Issue #4 (Credentials in CLI)
- **A07:2021 - Identification and Authentication Failures**: Issues #3, #4

### CWE/SANS Top 25
- CWE-295: Improper Certificate Validation (Issue #3)
- CWE-362: Concurrent Execution (Issue #2)
- CWE-20: Improper Input Validation (Issue #5)

---

## Testing Recommendations

1. **Fuzzing**: Use `go-fuzz` on the JSON parsing and WebSocket message handling
2. **Race Detection**: Run tests with `-race` flag
3. **Static Analysis**: Use `gosec`, `staticcheck`, and `go vet`
4. **Security Scanning**: Continue using Semgrep (already configured)

---

## Conclusion

The TrueNAS Go API Client is functional but has several security issues that should be addressed before production use, particularly:

1. **Fix all unsafe type assertions** (Critical)
2. **Add proper synchronization for job callbacks** (Critical)
3. **Remove or restrict InsecureSkipVerify option** (Critical)
4. **Move credentials to environment variables only** (High)
5. **Add input validation** (High)

The library shows good security awareness with Semgrep CI integration, but the runtime security could be significantly improved with the recommendations above.

---

## References

- [OWASP Top 10 2021](https://owasp.org/Top10/)
- [CWE Top 25](https://cwe.mitre.org/top25/)
- [Go Security Best Practices](https://go.dev/doc/security/best-practices)
- [Gorilla WebSocket Security](https://pkg.go.dev/github.com/gorilla/websocket#hdr-Security)
