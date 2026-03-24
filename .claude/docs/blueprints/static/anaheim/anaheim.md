# Anaheim API Usage Guide

The Anaheim API allows Applied employees to access Anaheim data. Version 1 endpoints are available at `https://anaheim.applied.co/applied/api/v1/*` and provide access to user and org chart data.

---

## Go Implementation (Copy-Paste Ready)

### Required Dependencies

```bash
go get google.golang.org/api/idtoken
go get golang.org/x/oauth2
```

### File Structure

```
anaheim/
├── types.go   # Data structures
├── auth.go    # Google OAuth ID token
└── client.go  # API client
```

### anaheim/types.go

```go
package anaheim

type Employee struct {
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	Email           string `json:"email"`
	TeamName        string `json:"teamName"`
	Title           string `json:"title"`
	ProfileImageURL string `json:"profileImageUrl"`
	ManagerEmail    string `json:"managerEmail"`
	Office          string `json:"office"`
	Timezone        string `json:"timezone"`
	GithubName      string `json:"githubName"`
	SlackID         string `json:"slackId"`
	LinkedinURL     string `json:"linkedinUrl"`
	JoinDate        string `json:"joinDate"`
	JobFamily       string `json:"jobFamily"`
}

type UsersResponse struct {
	Users []Employee `json:"users"`
}

type UserFilter struct {
	Emails        []string `json:"emails,omitempty"`
	Teams         []string `json:"teams,omitempty"`
	Titles        []string `json:"titles,omitempty"`
	ManagerEmails []string `json:"manager_emails,omitempty"`
}
```

### anaheim/auth.go

**IMPORTANT:** `idtoken.NewTokenSource` returns `oauth2.TokenSource`, NOT `*idtoken.TokenSource`.

```go
package anaheim

import (
	"context"
	"os"

	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

type GoogleAuthClient struct {
	tokenSource oauth2.TokenSource  // NOT *idtoken.TokenSource
}

func NewGoogleAuthClient(credentialsPath, targetAudience string) (*GoogleAuthClient, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, err
	}

	ts, err := idtoken.NewTokenSource(context.Background(), targetAudience,
		idtoken.WithCredentialsJSON(data))
	if err != nil {
		return nil, err
	}

	return &GoogleAuthClient{tokenSource: ts}, nil
}

func (c *GoogleAuthClient) GetToken(ctx context.Context) (string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}
```

### anaheim/client.go

```go
package anaheim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AnaheimClient struct {
	baseURL    string
	authClient *GoogleAuthClient
	httpClient *http.Client
}

func NewAnaheimClient(baseURL string, auth *GoogleAuthClient) *AnaheimClient {
	return &AnaheimClient{
		baseURL:    baseURL,
		authClient: auth,
		httpClient: &http.Client{},
	}
}

func (c *AnaheimClient) GetAllEmployees(ctx context.Context) ([]Employee, error) {
	return c.GetEmployeesByFilter(ctx, UserFilter{})
}

func (c *AnaheimClient) GetEmployeesByFilter(ctx context.Context, filter UserFilter) ([]Employee, error) {
	token, err := c.authClient.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	body, _ := json.Marshal(filter)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/applied/api/v1/user", bytes.NewReader(body))

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, respBody)
	}

	var result UsersResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Users, nil
}
```

### Usage in main.go

```go
authClient, err := anaheim.NewGoogleAuthClient(
	"anaheim/anaheim-api-general-user-key.json",  // Path to credentials
	"anaheim.applied.co",                          // Target audience - MUST be exactly this
)
if err != nil {
	log.Fatalf("Failed to init Anaheim auth: %v", err)
}

client := anaheim.NewAnaheimClient("https://anaheim.applied.co", authClient)
employees, err := client.GetAllEmployees(ctx)
```

---

## Common Pitfalls

1. **Wrong TokenSource type** - Use `oauth2.TokenSource` not `*idtoken.TokenSource`
2. **Missing go mod tidy** - Run `go mod tidy` after adding idtoken dependency
3. **Wrong credentials path** - Key file is at `anaheim/anaheim-api-general-user-key.json`
4. **No main.go in anaheim folder** - The anaheim folder must be `package anaheim`, not `package main`
5. **Frontend not built** - Run `npm run build` in frontend/ before `go build` if using `//go:embed`
6. **Stale binary** - Kill existing processes and run `go clean -cache` if routes don't match code

---

## Authenticating 

Both human and machines users must authenticate via the following steps: 

1. **Read the anaheim-api-general-user.json file** This JSON file contains credentials for a Google service account that is authorized to make requests to Anaheim API. The file is inside the ./anaheim folder.


2. **Request an OAuth ID token.** Using these credentials, you can request a short-lived Google OAuth ID token with a one-hour expiration. Google provides client libraries that simplify this process. 


3. **Obtain ID token programmatically.** The example below shows how to programmatically obtain an ID token from Google (Note: target_audience must be "anaheim.applied.co"): 



```python
from google.auth.transport.requests import Request
from google.oauth2 import service_account
import json

# Target audience must be Anaheim
TARGET_AUDIENCE = "anaheim.applied.co"

# load credentials from API Service Account Key in 1Password
service_account_info = json.load(...)

id_token_credentials = service_account.IDTokenCredentials.from_service_account_info(
    service_account_info, target_audience=TARGET_AUDIENCE
)

if __name__ == "__main__":
    # request ID Token
    id_token_credentials.refresh(Request())
    print(f"id_token: {id_token_credentials.token}")

```



4. **Include the token in headers.** All requests to the Anaheim API must include an ID token in the Authorization header. The token should be prefixed with Bearer. 



Example using cURL:

```bash
curl -H "Authorization: Bearer <ID_TOKEN>" \
"https://anaheim.applied.co/applied/api/v1/user/<user_email>"
```

---

API Endpoints (Last updated: 2026-01-21) 

Request User Data 

**Endpoint:**


`GET https://anaheim.applied.co/applied/api/v1/user/<user_email>` 

**cURL:**

```bash
curl -H "Authorization: Bearer <ID_TOKEN>" \
"https://anaheim.applied.co/applied/api/v1/user/john.smith@applied.co"
```



**Response:**

```json
{
  "firstName": "John",
  "lastName": "Smith",
  "email": "john.smith@applied.co",
  "teamName": "Anaheim",
  "timezone": "America/Los_Angeles",
  "githubName": "john-smith-ai",
  "slackId": "1234ABCD",
  "managerEmail": "samk@applied.co",
  "linkedinUrl": "https://www.linkedin.com/in/johnsmith/",
  "office": "SVL-WCAL-HQ",
  "joinDate": "August 25, 2025",
  "title": "Software Engineer",
  "profile_image_url": "https://d1skco41jdw5ji.cloudfront.net/autocompressed_employee_headshots/john_smith.webp"
}
```



---

Request/Filter for Multiple Users 

**Endpoint:**


`POST https://anaheim.applied.co/applied/api/v1/user` 

**Request body:**
The request body lets you filter users based on email, team, title, manager, and github name, e.g: 

```json
{
  "emails": [...],
  "teams": ["Anaheim", "People Ops"],
  "titles": ["Software Engineer", "Manager"],
  "manager_emails": [...],
  "github_names": ["john-smith-ai"]
}

```



**Filtering rules:** 

1. **Filters are combined using AND across fields.** This means a user must match all of the filters provided. 


* Example: `{"teams": ["Anaheim"], "titles": ["Manager"], "manager_emails": ["mwan@applied.co"]}` 


* Only users who are on the Anaheim team AND have the title Manager AND report to mwan@applied.co will be returned. 




2. **Lists within a filter use OR.** Multiple values in a single filter field are treated as alternatives. A user only needs to match one of them. 


* Example: `{"teams": ["Anaheim"], "titles": ["Manager", "Software Engineer"]}` 


* Users will be returned if they are: Team Anaheim AND (Title Manager OR Software Engineer). 




3. **Empty filters or missing fields.** If you omit a field or if a list is empty (`"titles": []`) it won't filter on that property. 



**cURL example:**

```bash
curl -X POST "https://anaheim.applied.co/applied/api/v1/user" \
-H "Authorization: Bearer <ID_TOKEN>" \
-H "Content-Type: application/json" \
-d '{
  "teams": ["Anaheim"],
  "titles": ["Manager", "Software Engineer"],
  "manager_emails": ["samkalnins@applied.co"]
}'

```



**Response:**

```json
{
  "users": [
    {
      "email": "christine.fang@applied.co",
      "firstName": "Christine",
      "lastName": "Fang",
      "githubName": "christine-fang-ai",
      "joinDate": "January 21, 2021",
      "linkedinUrl": "www.linkedin.com/in/...",
      "managerEmail": "mwan@applied.co",
      "office": "SVL-WCAL-HQ",
      "profileImageUrl": "https://d1skco41jdw5ji.cloudfront.net...",
      "slackId": "ASKLUOI123",
      "teamName": "Anaheim",
      "timezone": "America/Los_Angeles",
      "title": "Software Engineer"
    }
  ]
}

```



---

Get Current and Historical Managers 

Returns the email addresses of managers of a report within a specified period. 

**Endpoint:**


`POST https://anaheim.applied.co/applied/api/v1/reporting_data/managers` 

**Request body parameters:** 

| Parameter | Type | Required? | Description |
| --- | --- | --- | --- |
| `report_email` | String | Yes | The @applied.co email of the employee you are looking up. 
| `get_current` | Boolean | Yes | If true, ignores `start_date` and `end_date` and returns the user's current manager(s). 
| `start_date` | ISO 8601 (e.g., 2025-01-01T00:00:00Z) | No | The beginning of the search window (inclusive). 
| `end_date` | ISO 8601 | No | The end of the search window (inclusive). 

* **Open Start:** If `start_date` is empty, it returns every manager from the user's first day up until the `end_date`. 


* **Open End:** If `end_date` is empty, it returns every manager from the `start_date` up until today. 


* **Full History:** If neither date is provided (and `get_current` is false), it returns every manager the user has ever had. 



**cURL example:**

```bash
curl -X POST \
-H "Authorization: Bearer <ID_TOKEN>" \
-H "Content-Type: application/json" \
-d '{
  "report_email": "mwan@applied.co",
  "get_current": false,
  "start_date": "2025-01-01T00:00:00Z",
  "end_date": "2025-12-31T23:59:59Z"
}' \
"https://anaheim.applied.co/applied/api/v1/reporting_data/managers"

```



**Response:**

```json
["alan.anderson@applied.co", "nick@applied.co"]

```



---

Get User Manager Chain 

Returns an ordered list of email addresses of managers going up the manager chain at a single point in time. 

**Endpoint:**


`POST https://anaheim.applied.co/applied/api/v1/reporting_data/manager_chain` 

**Request body parameters:** 

| Parameter | Type | Required? | Description |
| --- | --- | --- | --- |
| `report_email` | String | Yes | The @applied.co email of the employee you are looking up. 
| `snapshot_date` | ISO 8601 | No | (e.g., 2025-01-01T00:00:00Z). If not provided, then is set to current date. 

**cURL example:**

```bash
curl -X POST \
-H "Authorization: Bearer <ID_TOKEN>" \
-H "Content-Type: application/json" \
-d '{
  "report_email": "christine.fang@applied.co",
  "snapshot_date": "2025-12-12T23:59:59Z"
}' \
"https://anaheim.applied.co/applied/api/v1/reporting_data/manager_chain"

```



**Response:**

```json
["mwan@applied.co", "samkalnins@applied.co","pl@applied.co"]

```



---

Get Direct Reports 

Returns the email addresses of all direct reports of a user within a specified time window. 

**Endpoint:**


`POST https://anaheim.applied.co/applied/api/v1/reporting_data/direct_reports` 

**Request body parameters:** 

| Parameter | Type | Required? | Description |
| --- | --- | --- | --- |
| `manager_email` | String | Yes | The @applied.co email of the employee you are looking up. 
| `get_current` | Boolean | Yes | If true, ignores dates and returns the current direct reports of the user. 
| `start_date` | ISO 8601 timestamp (String) | No | The beginning of the search window (inclusive). 
| `end_date` | ISO 8601 timestamp (String) | No | The end of the search window (inclusive). 

* **Open Start:** If `start_date` is empty, it returns every direct report from the user's first day up until the `end_date`. 


* **Open End:** If `end_date` is empty, it returns every direct report from the `start_date` up until today. 


* **Full History:** If neither date is provided, it returns everyone that has ever reported directly to user. 



**cURL example:**

```bash
curl -X POST \
-H "Authorization: Bearer <ID_TOKEN>" \
-H "Content-Type: application/json" \
-d '{
  "manager_email": "mwan@applied.co",
  "get_current": false,
  "start_date": "2025-01-01T00:00:00Z",
  "end_date": "2025-12-31T23:59:59Z"
}' \
"https://anaheim.applied.co/applied/api/v1/reporting_data/direct_reports"

```



**Response:**

```json
["carolyn.wang@applied.co", "christine.fang@applied.co", "ingrid.xu@applied.co", "kai-ming.ang@applied.co", "nick@applied.co", "tenn@applied.co"]

```



---

Get All Reports (including non-direct) 

Returns a list of email addresses of all reports of a user at a specified point in time. 

**Endpoint:**


`POST https://anaheim.applied.co/applied/api/v1/reporting_data/all_reports` 

**Request body parameters:** 

| Parameter | Type | Required? | Description |
| --- | --- | --- | --- |
| `manager_email` | String | Yes | The @applied.co email of the employee you are looking up. 
| `snapshot_date` | ISO 8601 timestamp | No | (e.g. "2025-12-12T00:00:00Z"). If not provided, then is set to current date. 

**cURL example:**

```bash
curl -X POST \
-H "Authorization: Bearer <ID_TOKEN>" \
-H "Content-Type: application/json" \
-d '{
  "manager_email": "mwan@applied.co",
  "snapshot_date": "2025-12-12T23:59:59Z"
}' \
"https://anaheim.applied.co/applied/api/v1/reporting_data/all_reports"

```



**Response:**

```json
["tenn@applied.co", "kai-ming.ang@applied.co", "ingrid.xu@applied.co", "christine.fang@applied.co"]

```



---

Resume data 

Returns the companies each user has worked at given a list of their emails. Data is parsed from resumes and is not sanitized. 

**Endpoint:**


`POST https://anaheim.applied.co/applied/api/v1/resume_data` 

**Request body:**

```json
{
  "emails": ["christine.fang@applied.co", "ingrid.xu@applied.co"]
}

```



**cURL example:**

```bash
curl -X POST \
-H "Authorization: Bearer <ID_TOKEN>" \
-H "Content-Type: application/json" \
-d '{
  "emails": ["christine.fang@applied.co", "ingrid.xu@applied.co"]
}' \
"https://anaheim.applied.co/applied/api/v1/resume_data"

```



**Response:**

```json
{
  "resume_data": [
    {
      "email": "christine.fang@applied.co",
      "company_names": ["Warner Brothers", "Netflix"]
    },
    {
      "email": "ingrid.xu@applied.co",
      "company_names": ["JPMorgan Chase", "Deloitte"]
    }
  ]
}

```