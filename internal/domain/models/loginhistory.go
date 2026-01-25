// internal/domain/models/loginhistory.go
package models

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import "time"

// LoginRecord captures a single successful login event.
// CreatedAt is indexed for recent-activity views.
type LoginRecord struct {
	UserID    string    `bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
	IP        string    `bson:"ip"`
	Provider  string    `bson:"provider"`
}
