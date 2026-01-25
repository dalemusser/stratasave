package home

import (
	"testing"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.uber.org/zap"
)

func TestNewHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)
	router := Routes(h)

	if router == nil {
		t.Fatal("Routes() returned nil")
	}
}

func TestHomeVM(t *testing.T) {
	// Test that HomeVM struct works as expected
	vm := HomeVM{
		LandingTitle: "Test Title",
		CanEdit:      true,
	}

	if vm.LandingTitle != "Test Title" {
		t.Errorf("LandingTitle = %q, want %q", vm.LandingTitle, "Test Title")
	}

	if !vm.CanEdit {
		t.Error("CanEdit should be true")
	}
}
