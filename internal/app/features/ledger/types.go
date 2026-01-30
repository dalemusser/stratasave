// internal/app/features/ledger/types.go
package ledgerfeature

import (
	ledgerstore "github.com/dalemusser/stratasave/internal/app/store/ledger"
	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// LedgerEntryVM is the view model for a single ledger entry.
type LedgerEntryVM struct {
	ID                 string
	RequestID          string
	TraceID            string
	ClientRequestID    string
	Method             string
	Path               string
	Query              string
	Headers            map[string]string
	RemoteIP           string
	ActorType          string
	ActorID            string
	ActorName          string
	RequestBodySize    int64
	RequestBodyHash    string
	RequestBodyPreview string
	RequestBody        string // Full body (only available on errors)
	RequestContentType string
	StatusCode         int
	ResponseSize       int64
	ErrorClass         string
	ErrorMessage       string
	DecodeMs           float64
	ValidateMs         float64
	DBQueryMs          float64
	EncodeMs           float64
	TotalMs            float64
	StartedAt          string
	CompletedAt        string
	StartedAtISO       string // ISO 8601 format for JavaScript timezone conversion
	CompletedAtISO     string // ISO 8601 format for JavaScript timezone conversion
	Duration           string
	Metadata           map[string]any
	StatusClass        string // CSS class for status code
}

// LedgerListVM is the view model for the ledger list page.
type LedgerListVM struct {
	viewdata.BaseVM
	TimezoneGroups []timezones.ZoneGroup
	Entries        []LedgerEntryVM
	Filter         ledgerstore.ListFilter
	Page           int
	TotalPages     int
	TotalCount     int64
	PrevPage       int
	NextPage       int
	Error          string
}

// LedgerDetailVM is the view model for the ledger detail page.
type LedgerDetailVM struct {
	viewdata.BaseVM
	TimezoneGroups []timezones.ZoneGroup
	Entry          LedgerEntryVM
}

// StatusBreakdownVM represents a status category with its count and percentage.
type StatusBreakdownVM struct {
	Status     string
	Count      int64
	Percentage int
}

// LedgerStatsVM is the view model for the ledger statistics page.
type LedgerStatsVM struct {
	viewdata.BaseVM
	StartDate        string
	EndDate          string
	TotalRequests    int64
	StatusCounts     map[string]int64
	StatusBreakdown  []StatusBreakdownVM
	TotalErrors      int64
	AvgResponseTime  float64
	RecentErrors     []LedgerEntryVM
}
