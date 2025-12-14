# T04 - Dashboard Authentication

**Backlog**: P4200 (Go Dashboard Core)
**Priority**: Must Have

---

## Purpose

Verify that the dashboard authenticates users securely with password and optional TOTP.

---

## Prerequisites

- Dashboard running with configured credentials
- `NIXFLEET_PASSWORD_HASH` set (bcrypt)
- `NIXFLEET_SESSION_SECRETS` set (for signed cookies)
- Optional: `NIXFLEET_TOTP_SECRET` for 2FA

---

## Scenarios

### Scenario 1: Successful Login (Password Only)

**Given** TOTP is not configured
**And** the user is on the login page
**When** the user enters the correct password
**And** submits the form
**Then** the user is redirected to the dashboard
**And** a signed session cookie is set
**And** the session is stored in the database

### Scenario 2: Successful Login (Password + TOTP)

**Given** TOTP is configured
**And** the user is on the login page
**When** the user enters the correct password
**And** enters a valid TOTP code
**And** submits the form
**Then** the user is redirected to the dashboard
**And** a signed session cookie is set

### Scenario 3: Failed Login (Wrong Password)

**Given** the user is on the login page
**When** the user enters an incorrect password
**And** submits the form
**Then** the user sees an error message "Invalid password"
**And** the user remains on the login page
**And** no session is created

### Scenario 4: Failed Login (Wrong TOTP)

**Given** TOTP is configured
**And** the user enters the correct password
**When** the user enters an invalid TOTP code
**And** submits the form
**Then** the user sees an error message "Invalid TOTP code"
**And** the user remains on the login page

### Scenario 5: Rate Limiting

**Given** the user has failed 5 login attempts in the last minute
**When** the user attempts another login
**Then** the request is rejected with 429 Too Many Requests
**And** the user must wait before trying again

### Scenario 6: Session Expiry

**Given** the user is logged in
**And** the session is 24+ hours old
**When** the user makes a request
**Then** the user is redirected to the login page
**And** the old session is deleted from the database

### Scenario 7: Logout

**Given** the user is logged in
**When** the user clicks logout
**And** the CSRF token is valid
**Then** the session is deleted from the database
**And** the session cookie is cleared
**And** the user is redirected to the login page

### Scenario 8: CSRF Protection

**Given** the user is logged in
**When** a POST request is made without a valid CSRF token
**Then** the request is rejected with 403 Forbidden
**And** the action is not performed

### Scenario 9: Unauthenticated Access

**Given** the user is not logged in
**When** the user tries to access the dashboard
**Then** the user is redirected to the login page

---

## Verification Commands

```bash
# Test login
curl -c cookies.txt -X POST http://localhost:8000/login \
  -d "password=testpassword"
# Should redirect to /

# Test authenticated request
curl -b cookies.txt http://localhost:8000/api/hosts
# Should return host list

# Test unauthenticated request
curl http://localhost:8000/api/hosts
# Should return 401 or redirect

# Test rate limiting (run 6 times quickly)
for i in {1..6}; do
  curl -X POST http://localhost:8000/login -d "password=wrong"
done
# 6th request should get 429
```

---

## Test Implementation

```go
// tests/integration/dashboard_test.go

func TestDashboardAuth_LoginSuccess(t *testing.T) {
    // POST correct password
    // Verify redirect to /
    // Verify cookie set
    // Verify session in DB
}

func TestDashboardAuth_LoginFailed(t *testing.T) {
    // POST wrong password
    // Verify error message
    // Verify no cookie
}

func TestDashboardAuth_RateLimit(t *testing.T) {
    // POST 6 wrong passwords quickly
    // Verify 429 on 6th attempt
}

func TestDashboardAuth_CSRF(t *testing.T) {
    // Login successfully
    // POST to command endpoint without CSRF
    // Verify 403
    // POST with valid CSRF
    // Verify success
}
```
