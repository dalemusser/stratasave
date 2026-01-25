// Package formutil provides helpers for form re-rendering with validation errors.
//
// When a form submission fails validation, the form should be re-rendered with:
// - The user's previously entered values (echoed back)
// - An error message explaining what went wrong
// - All the context data needed for the form (dropdowns, etc.)
//
// This package provides a Base struct that can be embedded in form data structs
// to handle the common fields, and helper functions to populate them.
//
// Example usage:
//
//	type newUserData struct {
//		formutil.Base
//		FullName string
//		Email    string
//	}
//
//	// In your handler:
//	data := newUserData{
//		Base: formutil.NewBase(r, db, "Add User", "/system-users"),
//		FullName: full,
//		Email: email,
//	}
//	data.Error = template.HTML("Email is required.")
//	templates.Render(w, r, "user_new", data)
package formutil

import (
	"html/template"
	"net/http"

	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"go.mongodb.org/mongo-driver/mongo"
)

// Base contains common fields for form pages that can be embedded in form data structs.
// It embeds viewdata.BaseVM for site settings and user context, and adds Error for form validation.
type Base struct {
	viewdata.BaseVM
	Error template.HTML
}

// NewBase creates a fully populated Base for a form page.
// This is the preferred way to create a Base for embedding in form view models.
func NewBase(r *http.Request, db *mongo.Database, title, backDefault string) Base {
	return Base{
		BaseVM: viewdata.NewBaseVM(r, db, title, backDefault),
	}
}

// SetError sets the error message on a Base struct.
// This is a convenience method for setting Error as template.HTML.
func (b *Base) SetError(msg string) {
	b.Error = template.HTML(msg)
}
