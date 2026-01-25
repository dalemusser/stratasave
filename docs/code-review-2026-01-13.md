# Code Review: Issues and Fixes

**Date:** January 13, 2026
**Reviewer:** Claude Code

## Executive Summary

A comprehensive code review of the strata codebase identified several security vulnerabilities, code quality issues, and a complete absence of test coverage. This document details the issues found, fixes implemented, and remaining work.

### Fixes Implemented

The code review has been finalized with:

**Security Fixes:**
- XSS vulnerabilities fixed in pages and settings features
- Input size validation added for content fields
- Race condition in invitation acceptance fixed (check-then-create → atomic create with error handling)

---

## Security Issues

### 1. XSS Vulnerability in Pages Feature (HIGH)

**File:** `internal/app/features/pages/pages.go`

**Issue:** Page content was converted directly to `template.HTML` without sanitization, allowing stored XSS attacks.

```go
// BEFORE (vulnerable)
vm.Content = template.HTML(page.Content)
```

**Fix:** Added HTML sanitization using the existing `htmlsanitize` package.

```go
// AFTER (secure)
vm.Content = htmlsanitize.PrepareForDisplay(page.Content)
```

Also sanitized content before storage:
```go
content := htmlsanitize.Sanitize(r.FormValue("content"))
```

**Files Modified:**
- `internal/app/features/pages/pages.go` (lines 11, 97, 199)

---

### 2. XSS Vulnerability in Settings Feature (HIGH)

**File:** `internal/app/features/settings/settings.go`

**Issue:** `FooterHTML` and `LandingContent` were stored and displayed without sanitization.

```go
// BEFORE (vulnerable)
vm.FooterHTML = template.HTML(settings.FooterHTML)
```

**Fix:** Added sanitization on both storage and display.

```go
// AFTER (secure)
landingContent := htmlsanitize.Sanitize(r.FormValue("landing_content"))
footerHTML := htmlsanitize.Sanitize(r.FormValue("footer_html"))
// ...
vm.FooterHTML = htmlsanitize.SanitizeToHTML(settings.FooterHTML)
```

**Files Modified:**
- `internal/app/features/settings/settings.go` (lines 15, 106, 127-128, 146-147, 223)

---

### 3. Missing Input Size Validation (MEDIUM)

**Files:** `pages/pages.go`, `settings/settings.go`

**Issue:** No maximum length validation on rich text content fields could lead to DoS attacks through extremely large payloads.

**Fix:** Added size limits before processing:

```go
// pages/pages.go
const MaxContentLength = 100000 // 100KB

if len(rawContent) > MaxContentLength {
    // Return error to user
}
```

```go
// settings/settings.go
const MaxContentLength = 100000 // 100KB
const MaxFooterLength = 10000   // 10KB

if len(rawLandingContent) > MaxContentLength {
    h.renderSettingsWithError(w, r, "Landing content is too long...")
    return
}
if len(rawFooterHTML) > MaxFooterLength {
    h.renderSettingsWithError(w, r, "Footer HTML is too long...")
    return
}
```

**Files Modified:**
- `internal/app/features/pages/pages.go` (lines 188-216)
- `internal/app/features/settings/settings.go` (lines 114-144)

---

## Code Quality Issues

### 4. Duplicated getClientIP Implementation

**Files:** `login/login.go`, `invitations/invitations.go`

**Issue:** Nearly identical IP extraction functions existed in two files, violating DRY principle.

**Fix:** Created a shared utility package.

**New File:** `internal/app/system/network/ip.go`

```go
package network

// GetClientIP extracts the client IP address from the request.
func GetClientIP(r *http.Request) string {
    // Check X-Forwarded-For header first (for reverse proxies)
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        if idx := strings.Index(xff, ","); idx != -1 {
            return strings.TrimSpace(xff[:idx])
        }
        return strings.TrimSpace(xff)
    }
    // Check X-Real-IP header
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    // Fall back to RemoteAddr, stripping the port
    if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
        return r.RemoteAddr[:idx]
    }
    return r.RemoteAddr
}
```

**Files Modified:**
- `internal/app/features/login/login.go` - Removed local function, now imports `network.GetClientIP`
- `internal/app/features/invitations/invitations.go` - Removed local function, now imports `network.GetClientIP`

---

## Test Infrastructure

### 5. Test Utilities Package

**Issue:** The codebase had zero test files across 47 packages.

**Fix:** Created a test utilities package following patterns from stratahub.

**New Files:**

`internal/testutil/db.go`
- `SetupTestDB(t *testing.T)` - Creates isolated test database with indexes
- `TestContext()` - Returns context with 30-second timeout
- Automatic cleanup via `t.Cleanup()`

`internal/testutil/http.go`
- `TestUser` struct and `AdminUser()` factory
- `WithUser()` - Injects user into request context
- `NewRequest()`, `NewAuthenticatedRequest()` - Request builders
- `ResponseRecorder` with assertion helpers

---

## Tests Added

### 6. Input Validation Tests

**File:** `internal/app/system/inputval/inputval_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestIsValidEmail` | 14 test cases for email validation |
| `TestIsValidHTTPURL` | 12 test cases for URL validation |
| `TestIsValidObjectID` | 9 test cases for MongoDB ObjectID validation |
| `TestIsValidAuthMethod` | 12 test cases for auth method validation |
| `TestValidate` | 4 test cases for struct validation |
| `TestResult_First` | Tests first error extraction |
| `TestResult_All` | Tests all errors concatenation |

---

### 7. Password Utility Tests

**File:** `internal/app/system/authutil/password_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestValidatePassword` | 17 test cases: length limits, common passwords |
| `TestHashPassword` | Verifies bcrypt hashing, salt uniqueness |
| `TestCheckPassword` | 5 test cases: correct/wrong passwords, edge cases |
| `TestPasswordRoundTrip` | End-to-end hash and verify for various inputs |
| `TestPasswordRules` | Verifies rules description |

---

### 8. User Store Tests

**File:** `internal/app/store/users/userstore_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestStore_Create` | Create user with normalization, timestamps |
| `TestStore_Create_InvalidRole` | Rejects invalid roles |
| `TestStore_Create_DuplicateLoginID` | Returns ErrDuplicateLoginID |
| `TestStore_GetByID` | Retrieves user by ObjectID |
| `TestStore_GetByID_NotFound` | Returns mongo.ErrNoDocuments |
| `TestStore_GetByLoginID` | Case-insensitive login ID lookup |
| `TestStore_Update` | Updates user fields |
| `TestStore_Delete` | Deletes user, returns count |
| `TestStore_Delete_NotFound` | Returns 0 for non-existent |
| `TestStore_CountActiveAdmins` | Counts active admin users |
| `TestStore_ExistsByLoginID` | Checks login ID existence |
| `TestStore_CreateFromInput` | Creates user from input struct |
| `TestStore_UpdateFromInput` | Partial updates via input struct |

---

### 9. Settings Store Tests

**File:** `internal/app/store/settings/settingsstore_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestStore_Get_DefaultSettings` | Returns defaults when none exist |
| `TestStore_Save_And_Get` | Save and retrieve settings |
| `TestStore_Save_Update` | Updates existing settings |
| `TestStore_Exists` | Checks if settings exist |
| `TestStore_Upsert` | Insert or update settings |
| `TestStore_Singleton` | Ensures only one settings document |

---

### 10. Auth Middleware Tests

**File:** `internal/app/system/auth/auth_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestNewSessionManager` | Valid/invalid config, weak keys |
| `TestSessionManager_SessionName` | Default and custom names |
| `TestCurrentUser` | Context user extraction |
| `TestSessionUser_UserID` | ObjectID conversion |
| `TestRequireSignedIn` | Auth requirement, redirects |
| `TestRequireRole` | Role-based access control |
| `TestRequireRole_MultipleRoles` | Multiple allowed roles |
| `TestIsDefaultKey` | Detects weak/default keys |
| `TestClassifySessionError` | Error categorization |
| `TestWantsHTML` | Accept header detection |

---

### 11. Network Utility Tests

**File:** `internal/app/system/network/ip_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestGetClientIP` | X-Forwarded-For, X-Real-IP, RemoteAddr |
| `TestGetClientIP_NoHeaders` | Fallback to RemoteAddr |
| `TestGetClientIP_ProxyChain` | Multiple proxy scenario |

---

### 12. HTML Sanitize Tests

**File:** `internal/app/system/htmlsanitize/htmlsanitize_test.go`

| Test Function | Description |
|--------------|-------------|
| `TestSanitize` | 12 cases: safe HTML, XSS removal |
| `TestSanitizeToHTML` | Returns template.HTML type |
| `TestIsPlainText` | Plain vs HTML detection |
| `TestPlainTextToHTML` | Converts text to safe HTML |
| `TestPrepareForDisplay` | Combined preparation |
| `TestSanitize_Idempotent` | Double sanitize gives same result |
| `TestSanitize_ListElements` | Preserves ul/li |
| `TestSanitize_FormattingElements` | Preserves formatting tags |

---

## Issues Identified But Not Fixed

The following issues were identified during the review but not addressed:

### 1. Missing CSRF Protection (MEDIUM)

**Description:** No CSRF token validation visible on POST handlers. While HTMX is used, explicit CSRF protection would improve security.

**Recommendation:** Add CSRF middleware to the router using gorilla/csrf or similar.

---

### 2. Untrusted X-Forwarded-For Header (LOW)

**File:** `internal/app/system/network/ip.go`

**Description:** The X-Forwarded-For header is trusted directly. An attacker behind a proxy could spoof their IP.

**Recommendation:** Add configuration for trusted proxy addresses or document the limitation.

---

### 3. Additional Test Coverage Needed

The following packages still have no test coverage:

**High Priority:**
- `internal/app/features/login` - Login flow

**Medium Priority:**
- `internal/app/store/sessions` - Session store
- `internal/app/store/invitation` - Invitation store
- Feature handlers (`systemusers`, `settings`, `invitations`)

---

## Files Changed Summary

| File | Change Type |
|------|-------------|
| `internal/app/features/pages/pages.go` | Modified - XSS fix, size validation |
| `internal/app/features/settings/settings.go` | Modified - XSS fix, size validation |
| `internal/app/features/login/login.go` | Modified - Use shared network utility |
| `internal/app/features/invitations/invitations.go` | Modified - Use shared network utility, fix race condition |
| `internal/app/system/network/ip.go` | **New** - Shared IP extraction utility |
| `internal/testutil/db.go` | **New** - Test database utilities |
| `internal/testutil/http.go` | **New** - HTTP test utilities |
| `internal/app/system/inputval/inputval_test.go` | **New** - Input validation tests |
| `internal/app/system/authutil/password_test.go` | **New** - Password utility tests |
| `internal/app/store/users/userstore_test.go` | **New** - User store tests |
| `internal/app/store/settings/settingsstore_test.go` | **New** - Settings store tests |
| `internal/app/system/auth/auth_test.go` | **New** - Auth middleware tests |
| `internal/app/system/network/ip_test.go` | **New** - Network utility tests |
| `internal/app/system/htmlsanitize/htmlsanitize_test.go` | **New** - HTML sanitize tests |

---

## Verification

### Build Verification
```bash
go build ./...  # Passes
```

### Test Verification
```bash
go test ./...
# ok  github.com/dalemusser/strata/internal/app/store/settings
# ok  github.com/dalemusser/strata/internal/app/store/users
# ok  github.com/dalemusser/strata/internal/app/system/auth
# ok  github.com/dalemusser/strata/internal/app/system/authutil
# ok  github.com/dalemusser/strata/internal/app/system/htmlsanitize
# ok  github.com/dalemusser/strata/internal/app/system/inputval
# ok  github.com/dalemusser/strata/internal/app/system/network
```

### Manual Testing Recommended
- Test page content editing with HTML containing `<script>` tags
- Test settings footer with malicious HTML
- Verify sanitized content renders correctly
- Test content length limits by submitting large payloads
