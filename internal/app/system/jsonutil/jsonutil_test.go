package jsonutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
		wantBody   string
	}{
		{
			name:       "200 OK with data",
			status:     http.StatusOK,
			data:       map[string]string{"message": "hello"},
			wantStatus: http.StatusOK,
			wantBody:   `{"message":"hello"}`,
		},
		{
			name:       "201 Created with data",
			status:     http.StatusCreated,
			data:       map[string]int{"id": 123},
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":123}`,
		},
		{
			name:       "nil data",
			status:     http.StatusOK,
			data:       nil,
			wantStatus: http.StatusOK,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			JSON(rec, tt.status, tt.data)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			body := strings.TrimSpace(rec.Body.String())
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestOK(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"status": "success"}
	OK(rec, data)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if got["status"] != "success" {
		t.Errorf("body status = %q, want success", got["status"])
	}
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]any{"id": 456, "created": true}
	Created(rec, data)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if got["id"].(float64) != 456 {
		t.Errorf("body id = %v, want 456", got["id"])
	}
}

func TestNoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	NoContent(rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body should be empty, got %q", rec.Body.String())
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		message    string
		wantStatus int
	}{
		{"bad request", http.StatusBadRequest, "invalid input", 400},
		{"not found", http.StatusNotFound, "resource not found", 404},
		{"internal error", http.StatusInternalServerError, "something went wrong", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Error(rec, tt.status, tt.message)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var got map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("json unmarshal error: %v", err)
			}
			if got["error"] != tt.message {
				t.Errorf("error = %q, want %q", got["error"], tt.message)
			}
		})
	}
}

func TestBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequest(rec, "invalid email")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if got["error"] != "invalid email" {
		t.Errorf("error = %q, want 'invalid email'", got["error"])
	}
}

func TestUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()
	Unauthorized(rec, "authentication required")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	var got map[string]string
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["error"] != "authentication required" {
		t.Errorf("error = %q, want 'authentication required'", got["error"])
	}
}

func TestForbidden(t *testing.T) {
	rec := httptest.NewRecorder()
	Forbidden(rec, "access denied")

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	var got map[string]string
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["error"] != "access denied" {
		t.Errorf("error = %q, want 'access denied'", got["error"])
	}
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound(rec, "user not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	var got map[string]string
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["error"] != "user not found" {
		t.Errorf("error = %q, want 'user not found'", got["error"])
	}
}

func TestInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	InternalError(rec, "internal server error")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}

	var got map[string]string
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["error"] != "internal server error" {
		t.Errorf("error = %q, want 'internal server error'", got["error"])
	}
}

func TestValidationError(t *testing.T) {
	rec := httptest.NewRecorder()
	errors := map[string]string{
		"email": "invalid email format",
		"name":  "required",
	}
	ValidationError(rec, errors)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	var got struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	if got.Error != "validation failed" {
		t.Errorf("error = %q, want 'validation failed'", got.Error)
	}
	if got.Fields["email"] != "invalid email format" {
		t.Errorf("fields.email = %q, want 'invalid email format'", got.Fields["email"])
	}
	if got.Fields["name"] != "required" {
		t.Errorf("fields.name = %q, want 'required'", got.Fields["name"])
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid JSON",
			body:    `{"name":"test","value":123}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			body:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))

			var got map[string]any
			err := Decode(req, &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecode_StructBinding(t *testing.T) {
	type Input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	body := `{"name":"John","email":"john@example.com","age":30}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	var input Input
	if err := Decode(req, &input); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if input.Name != "John" {
		t.Errorf("Name = %q, want 'John'", input.Name)
	}
	if input.Email != "john@example.com" {
		t.Errorf("Email = %q, want 'john@example.com'", input.Email)
	}
	if input.Age != 30 {
		t.Errorf("Age = %d, want 30", input.Age)
	}
}

func TestDecode_BodyConsumed(t *testing.T) {
	body := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	var first map[string]string
	if err := Decode(req, &first); err != nil {
		t.Fatalf("First Decode() error = %v", err)
	}

	// Body should be consumed, second decode should fail
	var second map[string]string
	err := Decode(req, &second)
	if err != io.EOF {
		t.Errorf("Second Decode() should fail with EOF, got %v", err)
	}
}

func TestJSON_ComplexData(t *testing.T) {
	rec := httptest.NewRecorder()

	data := map[string]any{
		"users": []map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
		"total": 2,
		"meta": map[string]any{
			"page":  1,
			"limit": 10,
		},
	}

	JSON(rec, http.StatusOK, data)

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	users := got["users"].([]any)
	if len(users) != 2 {
		t.Errorf("users length = %d, want 2", len(users))
	}
	if got["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", got["total"])
	}
}

func TestDecode_LargeBody(t *testing.T) {
	// Create a large JSON body
	var buf bytes.Buffer
	buf.WriteString(`{"items":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(`{"id":` + string(rune('0'+i%10)) + `}`)
	}
	buf.WriteString(`]}`)

	req := httptest.NewRequest(http.MethodPost, "/", &buf)

	var got map[string]any
	if err := Decode(req, &got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	items := got["items"].([]any)
	if len(items) != 1000 {
		t.Errorf("items length = %d, want 1000", len(items))
	}
}
