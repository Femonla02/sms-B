// File: main.go
// Mass SMS Mailer - Single file production-ready Go package
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SMTPConfig holds SMTP server configuration details.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	Enabled  bool   `json:"enabled"`
}

// SMTPPool manages multiple SMTP configurations with usage limits and rotation
type SMTPPool struct {
	Configs     []SMTPConfig
	UsageLimits []int
	Counters    []int
	Mutex       sync.Mutex
	CurrentIdx  int
}

// NewSMTPPool creates a new SMTP pool with configurations and usage limits
func NewSMTPPool(configs []SMTPConfig, usageLimits []int) *SMTPPool {
	// Ensure we have usage limits for all configs
	limits := make([]int, len(configs))
	for i := range configs {
		if i < len(usageLimits) {
			limits[i] = usageLimits[i]
		} else {
			limits[i] = 0 // 0 means no limit
		}
	}

	return &SMTPPool{
		Configs:     configs,
		UsageLimits: limits,
		Counters:    make([]int, len(configs)),
		CurrentIdx:  0,
	}
}

// GetNextSMTP returns the next available SMTP configuration considering usage limits
func (p *SMTPPool) GetNextSMTP() (SMTPConfig, bool) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	// Try all configs starting from current index
	for i := 0; i < len(p.Configs); i++ {
		idx := (p.CurrentIdx + i) % len(p.Configs)
		config := p.Configs[idx]

		// Skip disabled configs
		if !config.Enabled {
			continue
		}

		// Check if usage limit is reached (0 means no limit)
		if p.UsageLimits[idx] == 0 || p.Counters[idx] < p.UsageLimits[idx] {
			p.Counters[idx]++
			p.CurrentIdx = (idx + 1) % len(p.Configs)
			return config, true
		}
	}

	// No available SMTP configs
	return SMTPConfig{}, false
}

// ResetCounters resets all usage counters
func (p *SMTPPool) ResetCounters() {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	for i := range p.Counters {
		p.Counters[i] = 0
	}
}

// SendRequest represents the payload for sending SMS messages.
type SendRequest struct {
	APIKey      string       `json:"apiKey"`
	FromName    string       `json:"fromName"`
	Subject     string       `json:"subject"`
	Letter      string       `json:"letter"`
	SMTPConfigs []SMTPConfig `json:"smtpConfigs"`
	UsageLimits []int        `json:"usageLimits"`
	Carriers    []string     `json:"carriers"`
	Numbers     []string     `json:"numbers"`
}

// SMTPTestRequest represents the payload for testing an SMTP connection.
type SMTPTestRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

// APIKey represents an API key record stored in the database.
type APIKey struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents a log of an SMS send attempt.
type Session struct {
	ID        int       `json:"id"`
	Phone     string    `json:"phone"`
	SMTP      string    `json:"smtp"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

var db *sql.DB
var carriers map[string]string

func init() {
	log.Println("Loading carriers.json...")
	carriers = make(map[string]string)
	data, err := ioutil.ReadFile("carriers.json")
	if err != nil {
		log.Fatalf("Error reading carriers.json: %v", err)
	}
	if err := json.Unmarshal(data, &carriers); err != nil {
		log.Fatalf("Error parsing carriers.json: %v", err)
	}

	log.Println("Initializing SQLite database (sms.db)...")
	db, err = sql.Open("sqlite3", "sms.db")
	if err != nil {
		log.Fatalf("DB open error: %v", err)
	}

	// Create tables if they don't exist.
	createAPIKeys := `CREATE TABLE IF NOT EXISTS api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		key TEXT,
		created_at DATETIME
	);`
	_, err = db.Exec(createAPIKeys)
	if err != nil {
		log.Fatalf("DB init error (api_keys): %v", err)
	}

	createSessions := `CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phone TEXT,
		smtp TEXT,
		status TEXT,
		message TEXT,
		timestamp DATETIME
	);`
	_, err = db.Exec(createSessions)
	if err != nil {
		log.Fatalf("DB init error (sessions): %v", err)
	}

	// Ensure directories exist.
	os.MkdirAll("logs", 0755)
	os.MkdirAll("sessions", 0755)
}

func main() {
	// Log server start.
	log.Println("Starting Mass SMS Mailer API server on http://localhost:3000 ...")

	// Serve static files from the public directory
	fs := http.FileServer(http.Dir("public"))
	http.Handle("/", http.StripPrefix("/", fs))
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("public/css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("public/js"))))

	// Register API endpoints using the default HTTP mux.
	http.HandleFunc("/api/generate-api-key", GenerateAPIKeyHandler)
	http.HandleFunc("/api/test-smtp", TestSMTPHandler)
	http.HandleFunc("/api/test-smtp-config", TestSMTPConfigHandler)
	http.HandleFunc("/api/send", SendSMSHandler)
	http.HandleFunc("/api/keys", ListAPIKeysHandler)
	http.HandleFunc("/api/sms-status", GetSMSSessionHandler)

	// Start the HTTP server.
	serverAddr := ":3000"
	log.Printf("Server running at http://localhost%s", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
	// Log server stop (this line is unlikely to be reached).
	log.Println("Server stopped.")
}

// GenerateAPIKeyHandler handles API key generation requests.
func GenerateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	type Input struct {
		Name string `json:"name"`
	}
	var input Input
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// Generate a new API key.
	key := fmt.Sprintf("APIKEY-%d", rand.Int63())
	createdAt := time.Now()
	_, err := db.Exec("INSERT INTO api_keys (name, key, created_at) VALUES (?, ?, ?)", input.Name, key, createdAt)
	if err != nil {
		http.Error(w, "DB insert failed", http.StatusInternalServerError)
		return
	}
	response := APIKey{
		Name:      input.Name,
		Key:       key,
		CreatedAt: createdAt,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TestSMTPHandler handles SMTP test requests (legacy format).
func TestSMTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Println("ERROR: Invalid HTTP method. Expected POST.")
		return
	}

	var cfg SMTPTestRequest
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Println("ERROR: Failed to decode request body:", err)
		return
	}

	// Convert to SMTPConfig for testing
	smtpCfg := SMTPConfig{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
		Enabled:  true,
	}

	if err := testSMTPConfig(smtpCfg); err != nil {
		http.Error(w, "SMTP Test Failed: "+err.Error(), http.StatusInternalServerError)
		log.Printf("ERROR: SMTP Test failed for %s:%d - %v", cfg.Host, cfg.Port, err)
		return
	}

	w.Write([]byte("SMTP Test Passed"))
	log.Printf("SUCCESS: SMTP Test passed for %s:%d", cfg.Host, cfg.Port)
}

// TestSMTPConfigHandler handles SMTP test requests for individual configs
func TestSMTPConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg SMTPConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := testSMTPConfig(cfg); err != nil {
		http.Error(w, "SMTP Test Failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("SMTP Test Passed"))
}

func testSMTPConfig(cfg SMTPConfig) error {
	retries := 3
	for attempt := 1; attempt <= retries; attempt++ {
		log.Printf("Attempt %d/%d: Testing SMTP connection to %s:%d...", attempt, retries, cfg.Host, cfg.Port)

		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

		message := []byte("Subject: Test SMTP Connection\r\n\r\nThis is a test message.")

		err := smtp.SendMail(addr, auth, cfg.From, []string{cfg.From}, message)
		if err == nil {
			return nil
		}

		if attempt < retries {
			delay := time.Duration(attempt*2) * time.Second
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to connect to SMTP server after %d attempts", retries)
}

func testSMTPConnection(cfg SMTPTestRequest) error {
	smtpCfg := SMTPConfig{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
		Enabled:  true,
	}
	return testSMTPConfig(smtpCfg)
}

// SendSMSHandler handles SMS sending requests.
func SendSMSHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate SMTP configs
	if len(req.SMTPConfigs) == 0 {
		http.Error(w, "At least one SMTP configuration is required", http.StatusBadRequest)
		return
	}

	// Enable all SMTP configs by default
	for i := range req.SMTPConfigs {
		req.SMTPConfigs[i].Enabled = true
	}

	// Format phone numbers into email addresses using carrier gateways.
	var emails []string
	for _, carrier := range req.Carriers {
		domain, ok := carriers[carrier]
		if !ok {
			continue
		}
		for _, number := range req.Numbers {
			email := fmt.Sprintf("%s@%s", number, domain)
			emails = append(emails, email)
		}
	}

	// Create SMTP pool
	smtpPool := NewSMTPPool(req.SMTPConfigs, req.UsageLimits)

	// Send emails using the SMTP pool
	var logsSlice []string
	var failedEmails []string

	for _, email := range emails {
		config, ok := smtpPool.GetNextSMTP()
		if !ok {
			logEntry := fmt.Sprintf("To: %s - no available SMTP servers (all usage limits exceeded)", email)
			logsSlice = append(logsSlice, logEntry)
			failedEmails = append(failedEmails, email)
			continue
		}

		message := fmt.Sprintf("Subject: %s\r\nFrom: %s <%s>\r\n\r\n%s", req.Subject, req.FromName, config.From, req.Letter)
		auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
		addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

		err := smtp.SendMail(addr, auth, config.From, []string{email}, []byte(message))
		status := "sent"
		if err != nil {
			status = "failed"
			failedEmails = append(failedEmails, email)
			log.Printf("Failed to send to %s via %s: %v", email, config.Host, err)
		}

		logEntry := fmt.Sprintf("To: %s via %s - %s", email, config.Host, status)
		logsSlice = append(logsSlice, logEntry)

		// Insert a session log into the database.
		_, _ = db.Exec("INSERT INTO sessions (phone, smtp, status, message, timestamp) VALUES (?, ?, ?, ?, ?)",
			email, config.Host, status, req.Letter, time.Now())
	}

	// Save logs to file.
	logFileName := fmt.Sprintf("session_%d.txt", time.Now().Unix())
	logDir := "logs"
	_ = os.MkdirAll(logDir, 0755)
	err := ioutil.WriteFile(filepath.Join(logDir, logFileName), []byte(strings.Join(logsSlice, "\n")), 0644)
	if err != nil {
		log.Printf("Error writing log file: %v", err)
	}

	// Save JSON session data.
	sessFileName := fmt.Sprintf("session_%d.json", time.Now().Unix())
	sessDir := "sessions"
	_ = os.MkdirAll(sessDir, 0755)
	sessData, _ := json.MarshalIndent(req, "", "  ")
	_ = ioutil.WriteFile(filepath.Join(sessDir, sessFileName), sessData, 0644)

	// Prepare response
	response := map[string]interface{}{
		"status":       "completed",
		"total":        len(emails),
		"success":      len(emails) - len(failedEmails),
		"failed":       len(failedEmails),
		"failedEmails": failedEmails,
		"logFile":      logFileName,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListAPIKeysHandler lists all generated API keys.
func ListAPIKeysHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := db.Query("SELECT id, name, key, created_at FROM api_keys")
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(&key.ID, &key.Name, &key.Key, &key.CreatedAt); err != nil {
			continue
		}
		keys = append(keys, key)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// GetSMSSessionHandler retrieves a session by its ID (provided as a query parameter "id").
func GetSMSSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid session id", http.StatusBadRequest)
		return
	}
	row := db.QueryRow("SELECT id, phone, smtp, status, message, timestamp FROM sessions WHERE id = ?", id)
	var session Session
	if err := row.Scan(&session.ID, &session.Phone, &session.SMTP, &session.Status, &session.Message, &session.Timestamp); err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
