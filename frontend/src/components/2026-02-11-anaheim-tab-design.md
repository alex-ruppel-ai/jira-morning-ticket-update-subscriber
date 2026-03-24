# Anaheim Tab Implementation Design

**Goal:** Add a top-level "Anaheim" tab that provides employee directory search. Shows sample profiles when configured, setup message when not.

**Architecture:** New `Anaheim.tsx` component with search input. On mount, fetches two sample profiles (qy@applied.co, pl@applied.co) to verify configuration. Users can search by email or name to find employees. Backend exposes two endpoints: single user lookup and filtered user search.

**Tech Stack:** React 18, TypeScript, Tailwind CSS, Vite, Go with Gin framework

---

## Frontend Component

### Component Structure

**File:** `frontend/src/components/Anaheim.tsx`

**State:**
```typescript
const [profiles, setProfiles] = useState<FetchResult<Employee> | null>(null)
const [searchQuery, setSearchQuery] = useState('')
const [isSearching, setIsSearching] = useState(false)
```

**Employee Interface:**
```typescript
interface Employee {
  firstName: string
  lastName: string
  email: string
  teamName: string
  title: string
  profileImageUrl: string
  managerEmail: string
  office: string
  timezone: string
  githubName: string
  slackId: string
  linkedinUrl: string
  joinDate: string
  isActive: boolean
  memberType: string
}
```

### Fetch Logic

**Initial Load (Configuration Check):**
- On mount: fetch both sample profiles in parallel
  - `GET /api/anaheim/user/qy@applied.co`
  - `GET /api/anaheim/user/pl@applied.co`
- 404 or 400 → status: 'not_configured'
- 500 → status: 'error'
- Success → status: 'ok' with profiles

**Search:**
- User enters query and clicks "Search" or presses Enter
- Call `POST /api/anaheim/users` with `{query: searchQuery}`
- Backend parses query and searches email + name fields
- Results replace sample profiles

**FetchResult Type:**
```typescript
type FetchResult<T> =
  | { status: 'ok'; data: T[] }
  | { status: 'not_configured' }
  | { status: 'error' }
```

### Display States

**Loading:**
```
Anaheim
Employee directory

Loading...
```

**Not Configured:**
```
Anaheim
Employee directory

Anaheim is not configured. Follow the setup instructions in README.md to configure your Anaheim credentials.
```

**Error:**
```
Anaheim
Employee directory

Something went wrong. Please try again.
```

**Success:**
```
Anaheim
Employee directory

[Anaheim is connected] (green banner)

[Search input box] [Search button]

[Profile Card 1] [Profile Card 2]
[Profile Card 3] [Profile Card 4]
...
```

**No Search Results:**
```
No employees found matching your search
```

### Profile Card Layout

**Contents (from Employee type):**
- Circular profile image (profileImageUrl) - 64x64px, fallback to initials
- Full name (firstName + lastName) - bold, larger text
- Title - gray subtitle
- Email - with mailto link
- Team - labeled row
- Manager - labeled row, email format
- Office - labeled row
- Join Date - labeled row, formatted nicely
- LinkedIn - clickable icon/link (linkedinUrl)

**Styling:**
- White background with shadow
- Rounded corners
- Padding: 1rem
- 2-column grid on desktop, 1-column on mobile

---

## Backend Implementation

### Anaheim Client Initialization

**File:** `main.go`

**Location:** After Slack bot initialization (around line 50)

```go
// Initialize Anaheim client from Google Secret Manager
// Uses ANAHEIM_SECRET_NAME env var (defaults to "anaheim-credentials")
anaheimClient, err := anaheim.New(context.Background())
if err != nil {
    logger.Warn("failed to initialize anaheim client",
        zap.Error(err),
        zap.String("hint", "Set ANAHEIM_SECRET_NAME in .env and ensure secret exists in Secret Manager"),
    )
} else {
    logger.Info("anaheim client initialized successfully")
}
```

**Pass to route registration:**
```go
registerAPIRoutes(r, bot, anaheimClient)
```

### API Endpoints

**File:** `api.go`

**Endpoint 1: Single User Lookup**
```
GET /api/anaheim/user/:email
```

**Handler:**
- Extract email from URL params
- Call `anaheimClient.GetUserByEmail(ctx, email)`
- Response: `{success: true, user: Employee}`
- Error handling:
  - 404 if user not found
  - 500 on other errors

**Endpoint 2: User Search**
```
POST /api/anaheim/users
```

**Request Body:**
```json
{
  "query": "string"
}
```

**Handler:**
- Parse query string
- Build UserFilter:
  - Split query by whitespace
  - Add to both Emails and "name" search (use GetUsers with filter)
  - Note: Anaheim UserFilter doesn't have a Names field, so we'll need to search using the filter capabilities available (Emails, Teams, Titles, ManagerEmails, GithubNames)
  - For name search, we may need to fetch all users and filter client-side, or just search by email
- Call `anaheimClient.GetUsers(ctx, filter)`
- Response: `{success: true, users: []Employee}`
- Error handling:
  - 404 if no results
  - 500 on errors

**Route Registration:**
```go
if anaheimClient != nil {
    apiGroup.GET("/anaheim/user/:email", handleAnaheimGetUser)
    apiGroup.POST("/anaheim/users", handleAnaheimSearchUsers)
}
```

---

## Frontend Wiring

### App.tsx Changes

**Line 9:** Extend Tab type
```typescript
export type Tab = 'dashboard' | 'send-message' | 'send-dm' | 'members' | 'feedback' | 'anaheim'
```

**Import:** Add after other component imports
```typescript
import Anaheim from './components/Anaheim'
```

**Switch case:** Add in renderContent()
```typescript
case 'anaheim':
  return <Anaheim />
```

### Sidebar.tsx Changes

**Add tab entry:**
```typescript
{ id: 'anaheim', label: 'Anaheim', icon: '👥' },
```

---

## Configuration

**Environment Variable:**
- `ANAHEIM_SECRET_NAME` - name of secret in Google Cloud Secret Manager
- Defaults to "anaheim-credentials" if not specified
- Secret should contain Anaheim API credentials (client ID, client secret)

**Secret Format:**
- Stored in Secret Manager as JSON
- Format defined in `anaheim/auth.go`

**Setup Instructions:**
- Create secret in Google Cloud Secret Manager
- Set `ANAHEIM_SECRET_NAME` in `.env` file
- Set `PROJECT_ID` in `.env` file
- Restart application

---

## Error Handling

**Configuration States:**
- Not configured: 404/400 from endpoints → show setup message
- Server error: 500 from endpoints → show error message
- Success: 200 with data → show profiles

**Search Edge Cases:**
- Empty query: show validation message or keep sample profiles
- No results: "No employees found matching your search"
- Network error: "Something went wrong. Please try again."

**Profile Card Edge Cases:**
- Missing profile image: show initials (first letter of first + last name)
- Missing fields: show "N/A" or omit row
- Invalid dates: show raw string or "N/A"

---

## Implementation Notes

**Search Implementation Detail:**
- Anaheim UserFilter doesn't have a generic "Names" field
- Options:
  1. Search by Emails field only (email search)
  2. Make multiple API calls for different filter types
  3. Use GetUsers with empty filter, then filter client-side (if result set is small)
- **Decision:** Use Emails field for email search. For name search, may need to call GetUsers with empty filter and filter results in the backend handler.

**Sample Profiles:**
- qy@applied.co and pl@applied.co hardcoded in frontend
- Used only for initial configuration check
- Replaced by search results when user searches

**Styling:**
- Uses Tailwind utility classes
- Consistent spacing and colors

---

## Success Criteria

- [ ] Anaheim tab appears in sidebar
- [ ] Sample profiles (qy@ and pl@) display when configured
- [ ] Green "connected" banner shows on success
- [ ] Search input accepts queries and submits on Enter or button click
- [ ] Search results display in profile cards with all extended info
- [ ] Profile cards show: image, name, title, email, team, manager, office, join date, LinkedIn
- [ ] "Not configured" message shows when secret not set up
- [ ] Error messages clear and actionable
- [ ] Responsive layout works on mobile and desktop
