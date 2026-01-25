# Webhook/Event System Design

This document describes the design and implementation plan for adding a webhook/event system to Strata.

## Overview

A webhook/event system allows external services to be notified in real-time when events occur in Strata. Instead of polling for changes, external systems receive HTTP POST callbacks with event data.

### Goals

- Enable integration with external systems (CRMs, automation tools, etc.)
- Support automation workflows (Zapier, n8n, Make, custom scripts)
- Allow custom notification integrations (Slack, Teams, Discord)
- Provide audit trail for webhook deliveries
- Ensure reliable delivery with retries

### Non-Goals (Initial Release)

- Real-time websocket notifications (may be added later)
- Complex event filtering/transformation rules
- Fan-out to message queues (Kafka, RabbitMQ)

---

## Events

### Event Types

Events follow the pattern `{resource}.{action}`.

| Category | Event Type | Description |
|----------|------------|-------------|
| **Users** | `user.created` | New user account created (by admin or invitation) |
| | `user.updated` | User profile or settings updated |
| | `user.disabled` | User account disabled |
| | `user.enabled` | User account re-enabled |
| | `user.deleted` | User account deleted |
| **Authentication** | `user.login` | Successful login |
| | `user.logout` | User logged out |
| | `user.login_failed` | Failed login attempt |
| | `invitation.sent` | Invitation email sent |
| | `invitation.accepted` | Invitation accepted, user registered |
| | `invitation.revoked` | Invitation revoked by admin |
| **Admin** | `settings.updated` | Site settings changed |
| | `announcement.created` | New announcement created |
| | `announcement.updated` | Announcement updated |

### Event Payload Structure

All events follow a consistent structure:

```json
{
  "id": "evt_01HQ3V4X8K9M2N5P6R7S8T9U0V",
  "type": "user.created",
  "timestamp": "2026-01-19T15:30:00.000Z",
  "workspace_id": "507f1f77bcf86cd799439011",
  "data": {
    "user_id": "507f1f77bcf86cd799439012",
    "email": "newuser@example.com",
    "full_name": "New User",
    "role": "admin",
    "auth_method": "email"
  },
  "actor": {
    "user_id": "507f1f77bcf86cd799439013",
    "email": "admin@example.com"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique event ID (for idempotency) |
| `type` | string | Event type (e.g., `user.created`) |
| `timestamp` | string | ISO 8601 timestamp |
| `workspace_id` | string | Workspace where event occurred |
| `data` | object | Event-specific payload |
| `actor` | object | User who triggered the event (null for system events) |

---

## Database Schema

### webhooks Collection

Stores webhook endpoint configurations.

```
_id: ObjectID
workspace_id: ObjectID              // scoped to workspace
name: String                        // human-readable name
description: String | null          // optional description
url: String                         // endpoint URL (HTTPS required in prod)
secret: String                      // HMAC signing secret (auto-generated)
events: [String]                    // subscribed event types ["user.created", "user.*"]
enabled: Boolean                    // can be disabled without deleting
headers: Map[String, String] | null // custom headers to include
created_at: Timestamp
updated_at: Timestamp
created_by_id: ObjectID
created_by_name: String
```

**Indexes:**
- `idx_webhooks_workspace_enabled`: (workspace_id, enabled)
- `idx_webhooks_events`: (events) - for matching events to webhooks

### webhook_deliveries Collection

Stores delivery attempts for debugging and retry.

```
_id: ObjectID
webhook_id: ObjectID
event_id: String                    // matches event.id for correlation
event_type: String
payload: Document                   // full event payload
status: String                      // pending, delivered, failed, exhausted
attempts: Int32                     // number of delivery attempts
max_attempts: Int32                 // configured max (default 5)
next_attempt_at: Timestamp | null   // for retry scheduling
last_attempt_at: Timestamp | null
last_response_code: Int32 | null
last_response_body: String | null   // truncated to 1KB
last_error: String | null           // connection error, timeout, etc.
created_at: Timestamp
delivered_at: Timestamp | null
```

**Indexes:**
- `idx_deliveries_webhook_created`: (webhook_id, created_at desc)
- `idx_deliveries_status_next`: (status, next_attempt_at) - for retry job
- `idx_deliveries_ttl`: TTL index on created_at (30 days retention)

---

## Architecture

### Components

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Feature        │────▶│  Event          │────▶│  Webhook        │
│  Handlers       │     │  Emitter        │     │  Store          │
│                 │     │                 │     │                 │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                                 ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │                 │     │                 │
                        │  Delivery       │────▶│  External       │
                        │  Queue (Job)    │     │  Endpoints      │
                        │                 │     │                 │
                        └─────────────────┘     └─────────────────┘
```

### 1. Event Emitter

Central service for emitting events. Handlers call this to publish events.

```go
// internal/app/system/events/emitter.go
package events

type Event struct {
    Type        string
    WorkspaceID primitive.ObjectID
    Data        map[string]any
    ActorID     *primitive.ObjectID
}

type Emitter struct {
    webhookStore    *webhookstore.Store
    deliveryStore   *deliverystore.Store
    logger          *zap.Logger
}

func (e *Emitter) Emit(ctx context.Context, event Event) error {
    // 1. Generate event ID and timestamp
    // 2. Find matching webhooks for this event type
    // 3. Create delivery records for each webhook
    // 4. Delivery happens async via background job
}
```

### 2. Webhook Store

CRUD operations for webhook configurations.

```go
// internal/app/store/webhook/webhookstore.go
package webhookstore

type Store struct {
    c *mongo.Collection
}

func (s *Store) Create(ctx context.Context, input CreateInput) (*Webhook, error)
func (s *Store) Update(ctx context.Context, id primitive.ObjectID, input UpdateInput) error
func (s *Store) Delete(ctx context.Context, id primitive.ObjectID) error
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*Webhook, error)
func (s *Store) ListByWorkspace(ctx context.Context, workspaceID primitive.ObjectID) ([]Webhook, error)
func (s *Store) FindByEvent(ctx context.Context, workspaceID primitive.ObjectID, eventType string) ([]Webhook, error)
```

### 3. Delivery Store

Manages webhook delivery records and retry state.

```go
// internal/app/store/webhookdelivery/deliverystore.go
package deliverystore

func (s *Store) Create(ctx context.Context, delivery Delivery) error
func (s *Store) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string, response *DeliveryResponse) error
func (s *Store) GetPendingRetries(ctx context.Context, limit int) ([]Delivery, error)
func (s *Store) ListByWebhook(ctx context.Context, webhookID primitive.ObjectID, limit int) ([]Delivery, error)
```

### 4. Delivery Job

Background job that processes pending deliveries with retry logic.

```go
// internal/app/system/tasks/webhook_delivery.go
package tasks

func WebhookDeliveryJob(deliveryStore *deliverystore.Store, logger *zap.Logger) Job {
    return Job{
        Name:     "webhook-delivery",
        Interval: 10 * time.Second,
        Run: func(ctx context.Context) error {
            // 1. Get pending deliveries (status=pending or retry due)
            // 2. For each delivery:
            //    a. POST to webhook URL with signed payload
            //    b. Update delivery status based on response
            //    c. Schedule retry if failed (exponential backoff)
        },
    }
}
```

### 5. Webhook Feature (Admin UI)

Admin interface for managing webhooks.

```go
// internal/app/features/webhooks/webhooks.go
package webhooks

type Handler struct {
    webhookStore  *webhookstore.Store
    deliveryStore *deliverystore.Store
    errLog        *errorsfeature.ErrorLogger
    auditLogger   *auditlog.Logger
    logger        *zap.Logger
}

// Routes:
// GET  /webhooks           - list webhooks
// GET  /webhooks/new       - new webhook form
// POST /webhooks/new       - create webhook
// GET  /webhooks/{id}      - view webhook details + recent deliveries
// GET  /webhooks/{id}/edit - edit webhook form
// POST /webhooks/{id}      - update webhook
// POST /webhooks/{id}/delete - delete webhook
// POST /webhooks/{id}/test - send test event
```

---

## Security

### HMAC Signature Verification

All webhook payloads are signed using HMAC-SHA256. The signature is included in the `X-Webhook-Signature` header.

**Signature format:**
```
X-Webhook-Signature: sha256=<hex-encoded-signature>
```

**Verification (receiver side):**
```python
import hmac
import hashlib

def verify_signature(payload_body, signature_header, secret):
    expected = 'sha256=' + hmac.new(
        secret.encode(),
        payload_body,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature_header)
```

### Additional Security Headers

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Webhook-ID` | Webhook config ID | Identify which webhook |
| `X-Webhook-Event` | Event type | Quick filtering |
| `X-Webhook-Timestamp` | Unix timestamp | Replay attack prevention |
| `Content-Type` | `application/json` | Payload format |
| `User-Agent` | `Strata-Webhook/1.0` | Identify source |

### URL Validation

- HTTPS required in production (HTTP allowed in dev only)
- Block private/internal IP ranges (127.0.0.1, 10.x.x.x, 192.168.x.x, etc.)
- Block localhost and internal hostnames
- Validate URL format before saving

### Secret Generation

Webhook secrets are auto-generated using cryptographically secure random bytes:
```go
secret := make([]byte, 32)
crypto/rand.Read(secret)
webhookSecret := hex.EncodeToString(secret) // 64 char hex string
```

---

## Retry Logic

### Retry Strategy

- **Max attempts:** 5 (configurable per webhook)
- **Backoff:** Exponential with jitter
- **Timeout:** 30 seconds per request

| Attempt | Delay |
|---------|-------|
| 1 | Immediate |
| 2 | ~1 minute |
| 3 | ~5 minutes |
| 4 | ~30 minutes |
| 5 | ~2 hours |

### Success Criteria

A delivery is considered successful when:
- HTTP status code is 2xx (200-299)
- Response received within timeout

### Failure Handling

After max attempts exhausted:
- Status set to `exhausted`
- Webhook is NOT automatically disabled
- Admin can view failed deliveries and manually retry

---

## Admin UI

### Webhook List Page (`/webhooks`)

Displays all configured webhooks with:
- Name, URL (truncated), enabled status
- Subscribed event count
- Recent delivery success rate
- Actions: Edit, Delete, Test

### Webhook Detail Page (`/webhooks/{id}`)

Shows webhook configuration and recent deliveries:
- Full configuration details
- Secret (hidden, with reveal button)
- Last 20 deliveries with status
- Delivery details modal (request/response)

### New/Edit Webhook Form

Fields:
- Name (required)
- Description (optional)
- URL (required, validated)
- Events (multi-select checkboxes)
- Custom headers (key-value pairs)
- Enabled toggle

### Test Webhook

Send a test event to verify endpoint is working:
```json
{
  "id": "evt_test_...",
  "type": "webhook.test",
  "timestamp": "...",
  "data": {
    "message": "This is a test webhook delivery"
  }
}
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STRATASAVE_WEBHOOK_ENABLED` | `true` | Enable/disable webhook system |
| `STRATASAVE_WEBHOOK_MAX_ATTEMPTS` | `5` | Default max delivery attempts |
| `STRATASAVE_WEBHOOK_TIMEOUT` | `30s` | HTTP request timeout |
| `STRATASAVE_WEBHOOK_RETENTION` | `30d` | Delivery log retention period |

### Runtime Settings (Database)

| Setting | Default | Description |
|---------|---------|-------------|
| Webhooks enabled | `true` | Master switch for workspace |

---

## Implementation Plan

### Phase 1: Core Infrastructure

**Estimated scope:** Foundation for event emission and storage

1. Create `events` package with `Emitter` interface
2. Create `webhookstore` package with CRUD operations
3. Create `deliverystore` package for delivery records
4. Add database indexes via `EnsureSchema`
5. Create `Webhook` and `WebhookDelivery` domain models

**Files to create:**
- `internal/app/system/events/emitter.go`
- `internal/app/system/events/types.go`
- `internal/app/store/webhook/webhookstore.go`
- `internal/app/store/webhookdelivery/deliverystore.go`
- `internal/domain/models/webhook.go`

### Phase 2: Delivery System

**Estimated scope:** Background job for reliable delivery

1. Create HTTP client with timeout and retry logic
2. Implement HMAC signature generation
3. Create webhook delivery background job
4. Register job in `startup.go`
5. Add delivery status tracking

**Files to create:**
- `internal/app/system/tasks/webhook_delivery.go`
- `internal/app/system/webhookclient/client.go`

**Files to modify:**
- `internal/app/bootstrap/startup.go`

### Phase 3: Admin UI

**Estimated scope:** Web interface for managing webhooks

1. Create webhooks feature with handlers
2. Create list, detail, new, edit templates
3. Add routes to `routes.go`
4. Add navigation menu item

**Files to create:**
- `internal/app/features/webhooks/webhooks.go`
- `internal/app/features/webhooks/templates/list.gohtml`
- `internal/app/features/webhooks/templates/show.gohtml`
- `internal/app/features/webhooks/templates/new.gohtml`
- `internal/app/features/webhooks/templates/edit.gohtml`

**Files to modify:**
- `internal/app/bootstrap/routes.go`
- `internal/app/resources/templates/menu.gohtml`

### Phase 4: Event Integration

**Estimated scope:** Integrate event emission into existing handlers

1. Add `Emitter` to `DBDeps`
2. Inject `Emitter` into relevant handlers
3. Emit events from:
   - `systemusers` (create, update, disable, enable, delete)
   - `invitations` (sent, accepted, revoked)
   - `login` (success, failure, logout)
   - `settings` (updated)
   - `announcements` (created, updated)

**Files to modify:**
- `internal/app/bootstrap/dbdeps.go`
- `internal/app/bootstrap/startup.go`
- `internal/app/bootstrap/routes.go`
- `internal/app/features/systemusers/systemusers.go`
- `internal/app/features/invitations/invitations.go`
- `internal/app/features/login/login.go`
- `internal/app/features/logout/logout.go`
- `internal/app/features/settings/settings.go`
- `internal/app/features/announcements/announcements.go`

### Phase 5: Testing & Documentation

1. Unit tests for event emitter
2. Unit tests for webhook store
3. Integration tests for delivery job
4. Update `docs/configuration.md`
5. Update `docs/database-schema.md`
6. Create webhook API documentation for receivers

---

## Example: Receiving Webhooks

### Node.js/Express

```javascript
const crypto = require('crypto');
const express = require('express');
const app = express();

app.use(express.json({
  verify: (req, res, buf) => {
    req.rawBody = buf;
  }
}));

app.post('/webhook', (req, res) => {
  const signature = req.headers['x-webhook-signature'];
  const timestamp = req.headers['x-webhook-timestamp'];

  // Verify signature
  const expected = 'sha256=' + crypto
    .createHmac('sha256', process.env.WEBHOOK_SECRET)
    .update(req.rawBody)
    .digest('hex');

  if (!crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expected))) {
    return res.status(401).send('Invalid signature');
  }

  // Check timestamp (prevent replay attacks)
  const age = Date.now() / 1000 - parseInt(timestamp);
  if (age > 300) { // 5 minutes
    return res.status(401).send('Request too old');
  }

  // Process event
  const event = req.body;
  console.log(`Received ${event.type}:`, event.data);

  res.status(200).send('OK');
});
```

### Go

```go
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    signature := r.Header.Get("X-Webhook-Signature")

    // Verify signature
    mac := hmac.New(sha256.New, []byte(webhookSecret))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

    if !hmac.Equal([]byte(signature), []byte(expected)) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    var event WebhookEvent
    json.Unmarshal(body, &event)

    // Process event
    log.Printf("Received %s: %+v", event.Type, event.Data)

    w.WriteHeader(http.StatusOK)
}
```

---

## Future Enhancements

These are not included in the initial implementation but may be added later:

1. **Event filtering** - Filter events by field values (e.g., only `user.created` where `role=admin`)
2. **Transformation** - Transform payload before delivery (JSONPath, templates)
3. **Rate limiting** - Per-webhook rate limits to protect endpoints
4. **Batching** - Batch multiple events into single delivery
5. **Dead letter queue** - Store permanently failed events for manual review
6. **Websocket subscriptions** - Real-time event streaming
7. **Event replay** - Replay historical events to a webhook
8. **Webhook analytics** - Delivery success rates, latency metrics
