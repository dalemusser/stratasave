// internal/app/system/authz/authz.go
package authz

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"net/http"
	"strings"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserCtx returns the user's role (lowercased), name, Mongo ObjectID, and a found flag.
// If no user is present in context or the user ID is malformed, it returns
// "visitor", "", NilObjectID, false. This ensures callers can trust that
// ok=true means a valid, authenticated user with a valid ObjectID.
// The role is normalized to lowercase for consistent comparison.
func UserCtx(r *http.Request) (role string, name string, userID primitive.ObjectID, ok bool) {
	user, ok := auth.CurrentUser(r)
	if !ok {
		return "visitor", "", primitive.NilObjectID, false
	}
	userID, err := primitive.ObjectIDFromHex(user.ID)
	if err != nil {
		// Malformed user ID in session - fail closed for security.
		// This should not happen in normal operation; indicates session corruption.
		return "visitor", "", primitive.NilObjectID, false
	}
	return strings.ToLower(user.Role), user.Name, userID, true
}

// IsAdmin reports whether the current request's user is an admin.
func IsAdmin(r *http.Request) bool {
	role, _, _, ok := UserCtx(r)
	return ok && role == "admin"
}

// IsDeveloper reports whether the current request's user is a developer.
func IsDeveloper(r *http.Request) bool {
	role, _, _, ok := UserCtx(r)
	return ok && role == "developer"
}

// IsLoggedIn reports whether there is a user in the request context.
func IsLoggedIn(r *http.Request) bool {
	_, ok := auth.CurrentUser(r)
	return ok
}

// HasRole reports whether the current user has one of the specified roles.
func HasRole(r *http.Request, roles ...string) bool {
	role, _, _, ok := UserCtx(r)
	if !ok {
		return false
	}
	for _, allowed := range roles {
		if strings.ToLower(allowed) == role {
			return true
		}
	}
	return false
}

// ThemePreference returns the user's theme preference from the request context.
// Returns empty string if no user is logged in, which templates treat as "system".
func ThemePreference(r *http.Request) string {
	user, ok := auth.CurrentUser(r)
	if !ok {
		return ""
	}
	return user.ThemePreference
}
