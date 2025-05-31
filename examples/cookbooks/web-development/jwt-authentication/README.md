# Cookbook: JWT Authentication Implementation

## Overview
This cookbook demonstrates how to use CGE to add JWT (JSON Web Token) authentication to a Go web application. You'll learn how to plan, generate, and review the implementation of a complete authentication system.

## Prerequisites
- Go 1.21+
- CGE installed and configured
- Git repository initialized
- Basic understanding of HTTP servers and JWT concepts

## Starting Point
We begin with a simple Go web server that has no authentication. The server provides basic endpoints but lacks any security measures.

### Initial Project Structure
```
jwt-auth-demo/
├── go.mod
├── main.go          # Basic HTTP server
├── handlers.go      # Simple request handlers
└── README.md
```

## Step-by-Step Guide

### Step 1: Setup the Initial Project

First, let's create the starting codebase:

```bash
mkdir jwt-auth-demo
cd jwt-auth-demo

# Initialize Go module
go mod init jwt-auth-demo

# Create basic server files (see files below)
```

**go.mod**
```go
module jwt-auth-demo

go 1.21

require (
    github.com/gorilla/mux v1.8.0
)
```

**main.go**
```go
package main

import (
    "log"
    "net/http"
    
    "github.com/gorilla/mux"
)

func main() {
    r := mux.NewRouter()
    
    // Public endpoints
    r.HandleFunc("/", HomeHandler).Methods("GET")
    r.HandleFunc("/public", PublicHandler).Methods("GET")
    
    // Endpoints that need authentication (currently unprotected)
    r.HandleFunc("/profile", ProfileHandler).Methods("GET")
    r.HandleFunc("/admin", AdminHandler).Methods("GET")
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}
```

**handlers.go**
```go
package main

import (
    "encoding/json"
    "net/http"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
    response := map[string]string{
        "message": "Welcome to the API",
        "status":  "public",
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func PublicHandler(w http.ResponseWriter, r *http.Request) {
    response := map[string]string{
        "message": "This is a public endpoint",
        "access":  "unrestricted",
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
    // TODO: This should require authentication
    response := map[string]string{
        "message": "User profile data",
        "warning": "Currently unprotected!",
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func AdminHandler(w http.ResponseWriter, r *http.Request) {
    // TODO: This should require admin authentication
    response := map[string]string{
        "message": "Admin panel access",
        "warning": "Currently unprotected!",
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### Step 2: Planning with CGE

Now let's use CGE to plan the JWT authentication implementation:

```bash
# Generate a comprehensive plan for JWT authentication
./cge plan "Add JWT authentication system with login, token validation middleware, and role-based access control. Include user registration, login endpoints, and protect existing endpoints with appropriate authorization levels." --output jwt_plan.json
```

**Expected Plan Structure:**
The generated plan should include tasks like:
- Create user model and storage
- Implement JWT token generation and validation
- Add authentication middleware
- Create login and registration endpoints
- Protect existing endpoints with middleware
- Add role-based access control
- Implement token refresh mechanism
- Add comprehensive error handling

### Step 3: Review the Generated Plan

```bash
# Review the plan structure
cat jwt_plan.json | jq '.'

# Check the number of tasks
cat jwt_plan.json | jq '.tasks | length'

# Review specific tasks
cat jwt_plan.json | jq '.tasks[] | select(.name | contains("middleware"))'
```

### Step 4: Generate Implementation (Dry Run First)

```bash
# Preview what changes would be made
./cge generate --plan jwt_plan.json --dry-run

# Review the proposed changes carefully
# Look for:
# - New files to be created (auth.go, middleware.go, models.go)
# - Modifications to existing files (main.go, handlers.go)
# - Dependencies to be added (JWT library, bcrypt, etc.)
```

### Step 5: Apply the Generated Changes

```bash
# Apply the changes to implement JWT authentication
./cge generate --plan jwt_plan.json --apply

# Check what files were created/modified
git status
```

### Step 6: Review and Validate the Implementation

```bash
# Run CGE review to check for issues
./cge review --auto-fix --max-cycles 3

# Run tests to ensure everything works
go test ./...

# Check for any linting issues
golangci-lint run

# Test the server manually
go run .
```

### Step 7: Manual Testing

Test the authentication system:

```bash
# Test public endpoints (should work without auth)
curl http://localhost:8080/
curl http://localhost:8080/public

# Test registration
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass","role":"user"}'

# Test login
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}'

# Save the JWT token from login response
TOKEN="your_jwt_token_here"

# Test protected endpoints with token
curl http://localhost:8080/profile \
  -H "Authorization: Bearer $TOKEN"

# Test admin endpoint (should fail for regular user)
curl http://localhost:8080/admin \
  -H "Authorization: Bearer $TOKEN"
```

## Expected Results

After completing this cookbook, you should have:

### 1. **New Files Created:**
- `auth.go` - JWT token generation and validation functions
- `middleware.go` - Authentication and authorization middleware
- `models.go` - User model and data structures
- `storage.go` - In-memory user storage (for demo purposes)
- `auth_test.go` - Unit tests for authentication functions

### 2. **Modified Files:**
- `main.go` - Updated with new routes and middleware
- `handlers.go` - Enhanced with authentication checks
- `go.mod` - Added JWT and bcrypt dependencies

### 3. **New Endpoints:**
- `POST /register` - User registration
- `POST /login` - User authentication
- `POST /refresh` - Token refresh
- `POST /logout` - User logout

### 4. **Protected Endpoints:**
- `/profile` - Requires valid JWT token
- `/admin` - Requires admin role

### 5. **Security Features:**
- Password hashing with bcrypt
- JWT token generation and validation
- Role-based access control
- Token expiration handling
- Secure middleware implementation

## Validation Checklist

- [ ] Server starts without errors
- [ ] Public endpoints accessible without authentication
- [ ] User registration works correctly
- [ ] User login returns valid JWT token
- [ ] Protected endpoints require valid token
- [ ] Admin endpoints require admin role
- [ ] Invalid tokens are rejected
- [ ] Expired tokens are handled properly
- [ ] Passwords are properly hashed
- [ ] All tests pass
- [ ] No linting errors

## Troubleshooting

### Common Issues and Solutions

**Issue: "Module not found" errors**
```bash
# Solution: Download dependencies
go mod tidy
go mod download
```

**Issue: JWT token validation fails**
```bash
# Check if the secret key is consistent
# Verify token format in Authorization header
# Ensure token hasn't expired
```

**Issue: Password authentication fails**
```bash
# Verify bcrypt hashing is working
# Check password comparison logic
# Ensure user exists in storage
```

**Issue: Middleware not protecting endpoints**
```bash
# Verify middleware is applied to correct routes
# Check middleware order in main.go
# Ensure middleware returns proper HTTP status codes
```

**Issue: Role-based access not working**
```bash
# Verify role is included in JWT claims
# Check role validation logic in middleware
# Ensure user has correct role assigned
```

## Advanced Customizations

### 1. Database Integration
Replace in-memory storage with a real database:
```bash
./cge plan "Replace in-memory user storage with PostgreSQL database integration including migrations and connection pooling" --output db_plan.json
./cge generate --plan db_plan.json --apply
```

### 2. OAuth2 Integration
Add OAuth2 providers:
```bash
./cge plan "Add OAuth2 authentication with Google and GitHub providers alongside existing JWT system" --output oauth_plan.json
./cge generate --plan oauth_plan.json --apply
```

### 3. Rate Limiting
Add rate limiting to authentication endpoints:
```bash
./cge plan "Implement rate limiting for authentication endpoints to prevent brute force attacks" --output ratelimit_plan.json
./cge generate --plan ratelimit_plan.json --apply
```

## Learning Outcomes

By completing this cookbook, you've learned how to:

1. **Plan complex features** using CGE's planning capabilities
2. **Generate comprehensive implementations** with proper security practices
3. **Review and validate** generated code for quality and security
4. **Test authentication systems** manually and automatically
5. **Iterate and improve** implementations based on review feedback

## Next Steps

- Explore the [API Rate Limiting cookbook](../api-rate-limiting/README.md)
- Try the [Microservices Authentication cookbook](../../microservices/auth-service/README.md)
- Learn about [Database Integration patterns](../../data-processing/database-migration/README.md)

## Resources

- [JWT.io](https://jwt.io/) - JWT token debugger and documentation
- [Go JWT library documentation](https://github.com/golang-jwt/jwt)
- [bcrypt documentation](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
- [Gorilla Mux documentation](https://github.com/gorilla/mux) 