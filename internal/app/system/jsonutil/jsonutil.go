// Package jsonutil provides helper functions for JSON API responses.
//
// Use these helpers in API handlers to ensure consistent JSON responses
// with proper Content-Type headers and error formatting.
package jsonutil

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status code.
//
// Usage:
//
//	jsonutil.JSON(w, http.StatusOK, map[string]any{
//	    "status": "success",
//	    "data": result,
//	})
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// OK writes a 200 OK JSON response.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created writes a 201 Created JSON response.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent writes a 204 No Content response (no body).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error writes an error response with the given status code.
// The response body is {"error": message}.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// BadRequest writes a 400 Bad Request error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, message)
}

// Unauthorized writes a 401 Unauthorized error response.
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnauthorized, message)
}

// Forbidden writes a 403 Forbidden error response.
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, message)
}

// NotFound writes a 404 Not Found error response.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, message)
}

// InternalError writes a 500 Internal Server Error response.
// Use this for unexpected server errors. Do not expose internal details
// to clients - log the actual error separately.
func InternalError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, message)
}

// ValidationError writes a 400 Bad Request response with field-level errors.
//
// Usage:
//
//	jsonutil.ValidationError(w, map[string]string{
//	    "email": "invalid email format",
//	    "name":  "required",
//	})
func ValidationError(w http.ResponseWriter, errors map[string]string) {
	JSON(w, http.StatusBadRequest, map[string]any{
		"error":  "validation failed",
		"fields": errors,
	})
}

// Decode reads and decodes JSON from the request body into v.
// Returns an error that can be passed to BadRequest if decoding fails.
//
// Usage:
//
//	var input CreateLogInput
//	if err := jsonutil.Decode(r, &input); err != nil {
//	    jsonutil.BadRequest(w, err.Error())
//	    return
//	}
func Decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
