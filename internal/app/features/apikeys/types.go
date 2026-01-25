// internal/app/features/apikeys/types.go
package apikeysfeature

import "github.com/dalemusser/stratasave/internal/app/system/viewdata"

// ScopeVM is the view model for an API key scope.
type ScopeVM struct {
	Resource string
	Actions  []string
}

// APIKeyVM is the view model for a single API key.
type APIKeyVM struct {
	ID          string
	KeyPrefix   string
	Name        string
	Description string
	CreatedBy   string
	Status      string
	Scopes      []ScopeVM
	LastUsedAt  string
	UsageCount  int64
	CreatedAt   string
	UpdatedAt   string
	RevokedAt   string
	IsActive    bool
}

// APIKeyListVM is the view model for the API keys list page.
type APIKeyListVM struct {
	viewdata.BaseVM
	Keys  []APIKeyVM
	Error string
}

// APIKeyFormVM is the view model for API key create/edit forms.
type APIKeyFormVM struct {
	viewdata.BaseVM
	ID          string
	Name        string
	Description string
	Scopes      []ScopeVM
	IsEdit      bool
	IsActive    bool
	Error       string
}

// APIKeyCreatedVM is the view model shown after creating an API key.
type APIKeyCreatedVM struct {
	viewdata.BaseVM
	Key     APIKeyVM
	FullKey string // Full API key value (shown only once)
}

// APIKeyDetailVM is the view model for the API key detail page.
type APIKeyDetailVM struct {
	viewdata.BaseVM
	Key   APIKeyVM
	Error string
}

// APIKeyManageModalVM is the view model for the manage modal.
type APIKeyManageModalVM struct {
	viewdata.BaseVM
	Key     APIKeyVM
	BackURL string
}
