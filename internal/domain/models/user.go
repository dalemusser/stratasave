// internal/domain/models/user.go
package models

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents users of the application.
//
// Auth fields:
//   - LoginID: What the user types to identify themselves (stored lowercase)
//   - LoginIDCI: Case/diacritic-insensitive version for matching (folded)
//   - Email: Contact email (optional, stored lowercase)
//   - AuthMethod: google, email, password, trust
type User struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FullName   string             `bson:"full_name" json:"full_name"`
	FullNameCI string             `bson:"full_name_ci" json:"full_name_ci"` // lowercase, diacritics-stripped

	// Authentication fields
	LoginID   *string `bson:"login_id" json:"login_id"`       // User identifier (lowercase)
	LoginIDCI *string `bson:"login_id_ci" json:"login_id_ci"` // Folded for case/diacritic-insensitive matching
	Email     *string `bson:"email" json:"email"`             // Contact email (lowercase, optional)
	AuthMethod string `bson:"auth_method" json:"auth_method"` // google, email, password, trust

	// Password auth fields
	PasswordHash *string `bson:"password_hash,omitempty" json:"-"` // bcrypt hash (never in JSON)
	PasswordTemp *bool   `bson:"password_temp,omitempty" json:"-"` // true if must change on next login

	// Role and status
	Role   string `bson:"role" json:"role"`                      // admin (extensible: add more roles as needed)
	Status string `bson:"status,omitempty" json:"status,omitempty"` // active, disabled

	// User preferences
	ThemePreference string `bson:"theme_preference,omitempty" json:"theme_preference,omitempty"` // light, dark, system (empty = system)

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// User roles
const (
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
)

// AllRoles returns all valid user roles.
func AllRoles() []string {
	return []string{
		RoleAdmin,
		RoleDeveloper,
	}
}

// IsValidRole checks if a role is valid.
func IsValidRole(role string) bool {
	for _, r := range AllRoles() {
		if r == role {
			return true
		}
	}
	return false
}
