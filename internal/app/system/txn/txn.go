// Package txn provides transaction utilities for MongoDB and DocumentDB.
//
// This package simplifies running multi-document operations atomically with
// automatic fallback for environments that don't support transactions (e.g.,
// standalone MongoDB without replica set).
//
// Usage:
//
//	err := txn.Run(ctx, db, log, func(sc mongo.SessionContext) error {
//	    // All operations here are atomic
//	    if _, err := db.Collection("users").DeleteOne(sc, filter); err != nil {
//	        return err
//	    }
//	    return nil
//	})
//
// If transactions are not supported, the function runs without a transaction.
// This provides best-effort atomicity while remaining compatible with all
// MongoDB/DocumentDB configurations.
package txn

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Func is the function type for transaction operations.
// The function receives a context that may be a mongo.SessionContext (if in a
// transaction) or a regular context (if transactions are not supported).
type Func func(ctx context.Context) error

// Run executes the given function within a MongoDB transaction if possible.
// If transactions are not supported (standalone MongoDB, DocumentDB without
// replica set, etc.), it falls back to running the function without a transaction.
//
// The function receives a context that should be used for all database operations.
// When running in a transaction, this is a mongo.SessionContext; otherwise it's
// the original context.
//
// Parameters:
//   - ctx: Parent context for the operation
//   - db: MongoDB database handle
//   - log: Logger for warnings (can be nil to suppress warnings)
//   - fn: Function containing the database operations to run atomically
//
// Returns an error if the operations fail. If the transaction commits successfully
// or the fallback execution succeeds, returns nil.
func Run(ctx context.Context, db *mongo.Database, log *zap.Logger, fn Func) error {
	client := db.Client()

	session, err := client.StartSession()
	if err != nil {
		// Session creation failed - try without transaction
		if log != nil {
			log.Warn("failed to start session, running without transaction",
				zap.Error(err))
		}
		return fn(ctx)
	}
	defer session.EndSession(ctx)

	// Wrap the function to work with WithTransaction's expected signature
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		return nil, fn(sc)
	})

	if err != nil {
		if IsNotSupported(err) {
			// Transactions not supported - fall back to non-transactional execution
			if log != nil {
				log.Warn("transactions not supported, running without transaction",
					zap.Error(err))
			}
			return fn(ctx)
		}
		return err
	}

	return nil
}

// RunWithFallback is like Run but allows specifying a separate fallback function.
// This is useful when the transactional and non-transactional implementations
// differ (e.g., different error handling or logging).
//
// Parameters:
//   - ctx: Parent context for the operation
//   - db: MongoDB database handle
//   - log: Logger for warnings (can be nil)
//   - txnFn: Function to run in a transaction
//   - fallbackFn: Function to run if transactions are not supported
func RunWithFallback(ctx context.Context, db *mongo.Database, log *zap.Logger, txnFn, fallbackFn Func) error {
	client := db.Client()

	session, err := client.StartSession()
	if err != nil {
		if log != nil {
			log.Warn("failed to start session, using fallback",
				zap.Error(err))
		}
		return fallbackFn(ctx)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		return nil, txnFn(sc)
	})

	if err != nil {
		if IsNotSupported(err) {
			if log != nil {
				log.Warn("transactions not supported, using fallback",
					zap.Error(err))
			}
			return fallbackFn(ctx)
		}
		return err
	}

	return nil
}

// IsNotSupported checks if an error indicates that transactions are not supported.
// This detects:
//   - Standalone MongoDB without replica set
//   - DocumentDB with transactions disabled
//   - Other configurations that don't support multi-document transactions
//
// Known error codes:
//   - 20: "Transaction numbers are only allowed on a replica set member or mongos"
//   - 51: IllegalOperation
//   - 263: "Cannot run 'aggregate' in a multi-document transaction"
func IsNotSupported(err error) bool {
	if err == nil {
		return false
	}

	// Check for MongoDB command errors with known codes
	if cmdErr, ok := err.(mongo.CommandError); ok {
		switch cmdErr.Code {
		case 20, 51, 263:
			return true
		}
	}

	// Check error message for transaction-related failures.
	// This catches both MongoDB and DocumentDB error variations.
	errStr := strings.ToLower(err.Error())
	transactionKeywords := []string{
		"transaction",
		"replica set",
		"session",
		"not supported",
		"illegal operation",
	}

	matchCount := 0
	for _, kw := range transactionKeywords {
		if strings.Contains(errStr, kw) {
			matchCount++
		}
	}

	// Require at least 2 keyword matches to avoid false positives
	return matchCount >= 2
}
