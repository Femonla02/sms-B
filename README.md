
# SMS Mailer API

A production-ready HTTP API service for sending bulk SMS messages via email-to-SMS gateways using multiple SMTP servers with load balancing.

## Features

- 📧 Send SMS messages via email-to-SMS carrier gateways
- ⚖️ Load balancing across multiple SMTP servers 
- 🔄 Automatic SMTP server rotation with usage limits
- 🔑 API key authentication
- 📊 Session logging and status tracking
- 💾 SQLite database for persistent storage
- 🧪 SMTP connection testing
- 📝 Detailed logging to files

## Installation

### Prerequisites

- Go 1.23.2 or higher
- SQLite3

### Steps

1. Clone the repository:
```sh
git clone https://github.com/dwightsyeve/sms.git
cd sms-mailer
```

2. Install dependencies:
```sh
go mod download
```

3. Build the project:
```sh
go build -o sms-mailer
```

## Configuration

### Carrier Configuration

The `carriers.json` file contains mappings of carrier names to their email-to-SMS gateway domains. Example:

```json
{
  "Verizon": "vtext.com",
  "AT&T": "txt.att.net",
  "T-Mobile": "tmomail.net"
}
```

### SMTP Server Configuration

SMTP servers are configured via the API when sending messages. Each configuration includes:

```json
{
  "host": "smtp.example.com",
  "port": 587,
  "username": "user@example.com",
  "password": "password",
  "from": "sender@example.com"
}
```

## API Endpoints

### Generate API Key
- **POST** `/api/generate-api-key`
- Generates a new API key for authentication
- Request body:
```json
{
  "name": "Key Name"
}
```

### Test SMTP Configuration
- **POST** `/api/test-smtp-config`
- Tests connection to an SMTP server
- Request body: SMTP configuration object

### Send SMS Messages
- **POST** `/api/send`
- Sends SMS messages via email gateways
- Request body:
```json
{
  "apiKey": "APIKEY-xxx",
  "fromName": "Sender Name",
  "subject": "Message Subject",
  "letter": "Message content",
  "smtpConfigs": [
    {
      "host": "smtp.example.com",
      "port": 587,
      "username": "user",
      "password": "pass",
      "from": "sender@example.com"
    }
  ],
  "usageLimits": [100],
  "carriers": ["Verizon", "AT&T"],
  "numbers": ["1234567890"]
}
```

### List API Keys
- **GET** `/api/keys`
- Returns list of all generated API keys

### Get SMS Session Status
- **GET** `/api/sms-status?id={sessionId}`
- Retrieves status of a specific SMS sending session

## Database Schema

### API Keys Table
```sql
CREATE TABLE api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT,
  key TEXT,
  created_at DATETIME
);
```

### Sessions Table
```sql
CREATE TABLE sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  phone TEXT,
  smtp TEXT,
  status TEXT,
  message TEXT,
  timestamp DATETIME
);
```

## Directory Structure

```
├── main.go           # Main application code
├── carriers.json     # Carrier gateway configurations
├── go.mod           # Go module definition
├── go.sum           # Go module checksums
├── sms.db           # SQLite database
├── logs/            # Session log files
└── sessions/        # Session JSON files
```

## Logging

The application maintains two types of logs:

1. **Session Logs** (`logs/session_{timestamp}.txt`):
   - Records delivery status for each message
   - Format: `To: {email} via {smtp} - {status}`

2. **Session Data** (`sessions/session_{timestamp}.json`):
   - Complete session configuration and request data
   - Stored in JSON format

## Error Handling

The API uses standard HTTP status codes:

- 200: Success
- 400: Bad Request
- 401: Unauthorized
- 404: Not Found
- 405: Method Not Allowed
- 500: Internal Server Error

## Usage Example

```sh
# Generate an API key
curl -X POST http://localhost:3000/api/generate-api-key \
  -H "Content-Type: application/json" \
  -d '{"name": "My API Key"}'

# Send SMS messages
curl -X POST http://localhost:3000/api/send \
  -H "Content-Type: application/json" \
  -d @request.json
```

## Performance

- Concurrent SMTP connections
- Connection pooling
- Load balancing across SMTP servers
- Automatic retry mechanism for failed SMTP tests

## Security Considerations

- API key authentication required
- SMTP passwords stored in memory only
- No sensitive data logged to files
- SQLite database should be properly secured

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
