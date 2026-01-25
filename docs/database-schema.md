# Strata Database Schema Documentation

This document describes the MongoDB database schema used by Strata.

## Collections Overview

| Collection | Purpose |
|------------|---------|
| `users` | User accounts (admins, members) |
| `organizations` | Organizations within a workspace |
| `groups` | Groups/cohorts within organizations |
| `group_memberships` | Join table linking users to groups |
| `resources` | Content for members (games, surveys, tools) |
| `group_resource_assignments` | Assignment of resources to groups |
| `materials` | Content for leaders (guides, documentation) |
| `material_assignments` | Assignment of materials to orgs/leaders |
| `coordinator_assignments` | Links coordinators to organizations |
| `workspaces` | Top-level tenant containers |
| `pages` | Editable content pages |
| `login_records` | Login history for activity tracking |
| `sessions` | User sessions with activity tracking |
| `activity_events` | User activity events |
| `audit_events` | System audit log |
| `email_verifications` | Email verification tokens (TTL) |
| `oauth_states` | OAuth state tokens (TTL) |
| `site_settings` | Workspace-specific configuration |
| `announcements` | System announcements |

---

## Collection Schemas

### users

User accounts with authentication details.

```
_id: ObjectID
workspace_id: ObjectID | null      // null for super-admins
full_name: String
full_name_ci: String               // case-insensitive for sorting
login_id: String | null            // what user types to login
login_id_ci: String | null         // folded for matching
auth_return_id: String | null      // provider's return identifier
email: String | null
auth_method: String                // trust, password, email, google, etc.
password_hash: String | null       // bcrypt hash
password_temp: Boolean | null      // must change on next login
role: String                       // admin, analyst, coordinator, leader, member
status: String                     // active, disabled
organization_id: ObjectID | null   // for leaders/members
can_manage_materials: Boolean      // coordinator permission
can_manage_resources: Boolean      // coordinator permission
theme_preference: String           // light, dark, system
created_at: Timestamp
updated_at: Timestamp
```

**Indexes:**
- `uniq_users_workspace_login_auth`: Unique (workspace_id, login_id_ci, auth_method) - sparse
- `idx_users_login_auth`: (login_id_ci, auth_method)
- `idx_users_role_org_status_fullnameci_id`: (role, organization_id, status, full_name_ci, _id)
- `idx_users_role_status_fullnameci_id`: (role, status, full_name_ci, _id)
- `idx_users_org`: (organization_id)
- `idx_users_workspace_role_status`: (workspace_id, role, status)

---

### organizations

Organizations within a workspace.

```
_id: ObjectID
workspace_id: ObjectID
name: String
name_ci: String                    // case-insensitive
city: String
city_ci: String
state: String
state_ci: String
time_zone: String
contact_info: String
status: String
created_at: Timestamp
updated_at: Timestamp
```

**Indexes:**
- `uniq_orgs_workspace_nameci`: Unique (workspace_id, name_ci)
- `idx_orgs_workspace_status_nameci__id`: (workspace_id, status, name_ci, _id)
- `idx_orgs_nameci__id`: (name_ci, _id)
- `idx_orgs_cityci`: (city_ci)
- `idx_orgs_stateci`: (state_ci)

---

### groups

Groups/cohorts within organizations.

```
_id: ObjectID
workspace_id: ObjectID
name: String
name_ci: String
description: String
organization_id: ObjectID
status: String
created_at: Timestamp
updated_at: Timestamp
```

**Indexes:**
- `uniq_group_org_nameci`: Unique (organization_id, name_ci)
- `idx_groups_org`: (organization_id)
- `idx_groups_org_status_nameci__id`: (organization_id, status, name_ci, _id)

---

### group_memberships

Join table linking users to groups.

```
_id: ObjectID
workspace_id: ObjectID
group_id: ObjectID
user_id: ObjectID
org_id: ObjectID                   // denormalized for queries
role: String                       // leader, member
created_at: Timestamp
```

**Indexes:**
- `uniq_gm_user_group`: Unique (user_id, group_id)
- `idx_gm_group_role_user`: (group_id, role, user_id)
- `idx_gm_user_role_group`: (user_id, role, group_id)
- `idx_gm_org_role_group`: (org_id, role, group_id)

---

### resources

Content available to members.

```
_id: ObjectID
workspace_id: ObjectID
title: String
title_ci: String
subject: String | null
subject_ci: String | null
type: String                       // game, survey, tool
status: String                     // active, disabled
launch_url: String | null          // external URL
show_in_library: Boolean
file_path: String | null           // uploaded file path
file_name: String | null
file_size: Int64 | null
description: String | null
default_instructions: String | null
created_at: Timestamp
updated_at: Timestamp | null
created_by_id: ObjectID | null
created_by_name: String
updated_by_id: ObjectID | null
updated_by_name: String
```

**Indexes:**
- `uniq_resources_titleci`: Unique (title_ci)
- `idx_resources_status_titleci__id`: (status, title_ci, _id)
- `idx_resources_subjectci`: (subject_ci)
- `idx_resources_type`: (type)

---

### group_resource_assignments

Assignment of resources to groups with scheduling.

```
_id: ObjectID
workspace_id: ObjectID
group_id: ObjectID
organization_id: ObjectID
resource_id: ObjectID
visible_from: Timestamp | null
visible_until: Timestamp | null
instructions: String               // customizable per assignment
created_at: Timestamp
updated_at: Timestamp | null
created_by_id: ObjectID | null
created_by_name: String
updated_by_id: ObjectID | null
updated_by_name: String
```

**Indexes:**
- `idx_assign_group_resource`: (group_id, resource_id)
- `idx_assign_group`: (group_id)
- `idx_assign_resource`: (resource_id)
- `idx_assign_group_created`: (group_id, created_at desc)

---

### materials

Content for leaders.

```
_id: ObjectID
workspace_id: ObjectID
title: String
title_ci: String
subject: String | null
subject_ci: String | null
type: String                       // document, survey, guide
status: String
launch_url: String | null
file_path: String | null
file_name: String | null
file_size: Int64 | null
description: String | null
default_instructions: String | null
created_at: Timestamp
updated_at: Timestamp | null
created_by_id: ObjectID | null
created_by_name: String
updated_by_id: ObjectID | null
updated_by_name: String
```

**Indexes:**
- `uniq_materials_titleci`: Unique (title_ci)
- `idx_materials_status_titleci__id`: (status, title_ci, _id)
- `idx_materials_type`: (type)

---

### material_assignments

Assignment of materials to organizations or individual leaders.

```
_id: ObjectID
workspace_id: ObjectID
material_id: ObjectID
organization_id: ObjectID | null   // one of org or leader must be set
leader_id: ObjectID | null
visible_from: Timestamp | null
visible_until: Timestamp | null
directions: String
created_at: Timestamp
updated_at: Timestamp | null
created_by_id: ObjectID | null
created_by_name: String
updated_by_id: ObjectID | null
updated_by_name: String
```

**Indexes:**
- `idx_matassign_org`: (organization_id)
- `idx_matassign_leader`: (leader_id)
- `idx_matassign_material`: (material_id)

---

### coordinator_assignments

Links coordinators to organizations they manage.

```
_id: ObjectID
user_id: ObjectID
organization_id: ObjectID
created_at: Timestamp
created_by_id: ObjectID
created_by_name: String
```

**Indexes:**
- `uniq_coordassign_user_org`: Unique (user_id, organization_id)
- `idx_coordassign_user`: (user_id)
- `idx_coordassign_org`: (organization_id)

---

### workspaces

Top-level tenant containers.

```
_id: ObjectID
name: String
name_ci: String
subdomain: String                  // unique, e.g., "mhs"
logo_path: String | null
logo_name: String | null
status: String                     // active, disabled
created_at: Timestamp
updated_at: Timestamp
```

**Indexes:**
- `uniq_workspaces_subdomain`: Unique (subdomain)
- `uniq_workspaces_nameci`: Unique (name_ci)
- `idx_workspaces_status_nameci__id`: (status, name_ci, _id)

---

### pages

Editable content pages.

```
_id: ObjectID
slug: String                       // about, contact, terms-of-service, privacy-policy
title: String
content: String                    // HTML content
updated_at: Timestamp | null
updated_by_id: ObjectID | null
updated_by_name: String
```

**Indexes:**
- `uniq_pages_slug`: Unique (slug)

---

### sessions

User sessions for activity monitoring.

```
_id: ObjectID
token: String                      // unique 32-byte random token
user_id: ObjectID
organization_id: ObjectID | null
ip: String
user_agent: String | null
current_page: String | null
login_at: Timestamp
logout_at: Timestamp | null        // nil if active
last_activity: Timestamp
last_user_activity: Timestamp
end_reason: String | null          // logout, expired, inactive, admin_terminated
duration_secs: Int64 | null
created_by: String                 // login, heartbeat
expires_at: Timestamp
created_at: Timestamp
updated_at: Timestamp
```

**Indexes:**
- `idx_session_token`: Unique (token) - sparse
- `idx_session_user`: (user_id)
- `idx_session_ttl`: TTL on expires_at
- `idx_session_active`: (logout_at, last_activity desc)

---

### activity_events

User activity events for analytics.

```
_id: ObjectID
user_id: ObjectID
session_id: ObjectID
organization_id: ObjectID | null
timestamp: Timestamp
event_type: String                 // resource_launch, page_view
resource_id: ObjectID | null
resource_name: String | null
page_path: String | null
details: Map[String, Any] | null
```

**Indexes:**
- `idx_activity_session`: (session_id, timestamp)
- `idx_activity_user`: (user_id, timestamp desc)
- `idx_activity_org`: (organization_id, timestamp desc)
- `idx_activity_resource`: (resource_id, timestamp desc)

---

### audit_events

System audit log for compliance.

```
_id: ObjectID
timestamp: Timestamp
organization_id: ObjectID | null
category: String                   // auth, admin, security
event_type: String
user_id: ObjectID | null           // affected user
actor_id: ObjectID | null          // who performed action
ip: String
user_agent: String | null
success: Boolean
failure_reason: String | null
details: Map[String, String] | null
```

**Indexes:**
- (timestamp desc)
- (organization_id, timestamp desc)
- (user_id, timestamp desc)
- (category, event_type, timestamp desc)

**Event Types:**
- Auth: login_success, login_failed_*, logout, password_changed, verification_code_*
- Admin: user_*, group_*, org_*, resource_*, material_*, *_assigned, *_unassigned

---

### email_verifications

Email verification tokens with auto-cleanup.

```
_id: ObjectID
token: String
user_id: ObjectID
expires_at: Timestamp              // TTL index
```

**Indexes:**
- `idx_emailverify_expires_ttl`: TTL on expires_at
- `idx_emailverify_token`: (token)
- `idx_emailverify_user`: (user_id)

---

### oauth_states

OAuth state tokens with auto-cleanup.

```
state: String
return_url: String | null
expires_at: Timestamp              // TTL index
created_at: Timestamp
```

**Indexes:**
- `idx_oauth_state`: Unique (state)
- `idx_oauth_ttl`: TTL on expires_at

---

### site_settings

Workspace-specific configuration.

```
_id: ObjectID
workspace_id: ObjectID             // unique per workspace
site_name: String
logo_path: String | null
logo_name: String | null
landing_title: String | null
landing_content: String | null     // HTML
footer_html: String | null
enabled_auth_methods: [String] | null
notify_user_on_create: Boolean     // send welcome email when admin creates user
notify_user_on_disable: Boolean    // send notification when account disabled
notify_user_on_enable: Boolean     // send notification when account enabled
notify_user_on_welcome: Boolean    // send welcome email after invitation accepted
updated_at: Timestamp | null
updated_by_id: ObjectID | null
updated_by_name: String
```

**Indexes:**
- `uniq_sitesettings_workspace`: Unique (workspace_id)

---

### announcements

System announcements.

```
_id: ObjectID
title: String
content: String
type: String                       // info, warning, critical
dismissible: Boolean
active: Boolean
starts_at: Timestamp | null
ends_at: Timestamp | null
created_at: Timestamp
updated_at: Timestamp
```

**Indexes:**
- (active)
- (starts_at)
- (ends_at)

---

## Schema Patterns

### Case-Insensitive Fields

Fields with `_ci` suffix store folded values for case/diacritic-insensitive operations:
- `full_name_ci`, `login_id_ci` on users
- `name_ci`, `city_ci`, `state_ci` on organizations
- `title_ci`, `subject_ci` on resources/materials

### TTL Indexes

Auto-expiring documents:
- `email_verifications.expires_at` - verification codes
- `oauth_states.expires_at` - OAuth state tokens
- `sessions.expires_at` - session cleanup

### Denormalized Fields

For performance optimization:
- `created_by_name`, `updated_by_name` - avoid user lookups
- `org_id` on group_memberships - query optimization

### Workspace Scoping

All entities (except pages, announcements) have `workspace_id` for multi-tenant isolation.

---

## Relationships

```
workspace (1) ─────┬──── (*) organizations
                   ├──── (*) groups
                   ├──── (*) users
                   ├──── (*) resources
                   ├──── (*) materials
                   └──── (1) site_settings

organization (1) ──┬──── (*) groups
                   ├──── (*) users (leaders/members)
                   └──── (*) coordinator_assignments

group (1) ─────────┬──── (*) group_memberships
                   └──── (*) group_resource_assignments

user (1) ──────────┬──── (*) group_memberships
                   ├──── (*) sessions
                   ├──── (*) activity_events
                   └──── (*) audit_events
```

---

## Authentication Methods

Supported auth methods (configurable per workspace):
- `trust` - No password required
- `password` - Username/password
- `email` - Email verification code
- `google` - Google OAuth
- `microsoft` - Microsoft OAuth
- `clever` - Clever SSO
- `classlink` - ClassLink SSO
- `schoology` - Schoology SSO
