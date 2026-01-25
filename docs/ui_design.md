# UI Design Guide

Quick reference for implementing consistent UI layouts in StrataSave features. For comprehensive patterns, see `ui-design-patterns.md`.

## Page Types

StrataSave features have four main page types:

| Page Type | Purpose | Layout |
|-----------|---------|--------|
| **List** | Display items with Manage buttons | Full-width table with search/filter |
| **View/Detail** | Read-only display of a single item | Full-width with "Edit" button at bottom |
| **Edit** | Form to modify an item | Full-width form with Save/Cancel and Danger Zone |
| **New/Create** | Form to create a new item | Full-width form with Create/Cancel |

---

## Core Layout Structure

**Every page** uses this outer structure:

```html
{{ define "content" }}
<div class="flex flex-col h-full">
  <!-- Header row -->
  <div class="mb-4 flex items-center">
    <a href="{{ .BackURL }}"
       class="text-sm px-3 py-1 border dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 mr-2 no-loader"
       title="Go back">
      ← Back
    </a>
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Icon Page Title</h1>
  </div>

  <!-- Main content box - fills available space -->
  <div class="p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4">
    <!-- Content here -->
  </div>
</div>
{{ end }}
```

**Key CSS Classes:**
- **Outer container**: `flex flex-col h-full`
- **Content box**: `p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4`
- **Form fields width**: `max-w-xl` (inside the content box, not as page container)

**Important:** Use `mb-4` on the content box to match the layout's `py-4` padding. This ensures uniform spacing above the footer.

**Common Mistake:** Using `max-w-2xl mx-auto` as the outer container. This centers content narrowly. Use `flex flex-col h-full` instead.

---

## List Page Pattern

List pages display items in a table with Manage buttons. The header (title + primary action) is outside the content box.

```html
{{ define "content" }}
<div class="flex flex-col h-full">
  <!-- Header - OUTSIDE the content box -->
  <div class="mb-4 flex items-center justify-between">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">🔑 Items</h1>
    <a href="/items/new" class="px-4 py-2 bg-indigo-600 text-white rounded hover:bg-indigo-700 text-sm">Create Item</a>
  </div>

  <!-- Content box with table - note p-4 padding creates space around table -->
  <div class="p-4 bg-white dark:bg-gray-800 rounded shadow flex-1 mb-4 overflow-auto">
    {{ if .Items }}
    <table class="min-w-full text-sm text-left text-gray-700 dark:text-gray-300">
      <thead class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 uppercase text-xs sticky top-0 z-10">
        <tr class="border-b border-gray-300 dark:border-gray-600">
          <th class="px-4 py-3">Name</th>
          <th class="px-4 py-3">Status</th>
          <th class="px-4 py-3 text-right">Actions</th>
        </tr>
      </thead>
      <tbody>
        {{ range .Items }}
        <tr class="border-b border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-900/50">
          <td class="px-4 py-3">{{ .Name }}</td>
          <td class="px-4 py-3">
            <span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-400">Active</span>
          </td>
          <td class="px-4 py-3 text-right">
            <form hx-get="/items/{{ .ID }}/manage_modal" hx-target="#modal-root" hx-swap="innerHTML">
              <button type="submit" class="bg-indigo-600 text-white px-2 py-1 rounded text-xs hover:bg-indigo-700">Manage</button>
            </form>
          </td>
        </tr>
        {{ end }}
      </tbody>
    </table>
    {{ else }}
    <div class="p-8 text-center">
      <p class="text-gray-500 dark:text-gray-400 mb-4">No items found.</p>
      <a href="/items/new" class="px-4 py-2 bg-indigo-600 text-white rounded hover:bg-indigo-700 text-sm">Create Your First Item</a>
    </div>
    {{ end }}
  </div>
</div>
<div id="modal-root"></div>
{{ end }}
```

**Key Points:**
- Header (title + button) is **outside** the content box
- Content box has `p-4` padding - creates visual space around the table inside the dark box
- Content box has `mb-4` for consistent footer spacing
- Use `overflow-auto` for scrollable tables
- Include `<div id="modal-root"></div>` for manage modals

**Common Mistake:** Missing `p-4` on the content box makes the table header flush with the top of the dark box. Always include `p-4` for consistent padding.

---

## View/Detail Page Pattern

The detail page shows read-only information with an "Edit" button at the bottom.

```html
{{ define "content" }}
<div class="flex flex-col h-full">
  <!-- Header with back button -->
  <div class="mb-4 flex items-center">
    <a href="/items" class="text-sm px-3 py-1 border dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 mr-2 no-loader">← Back</a>
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">🔑 {{ .Item.Name }}</h1>
  </div>

  <!-- Main content box -->
  <div class="p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4">
    <div class="space-y-4 max-w-xl">
      <!-- Read-only fields -->
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
        <input type="text" value="{{ .Item.Name }}" readonly
               class="w-full border dark:border-gray-600 p-2 rounded bg-gray-50 dark:bg-gray-700 dark:text-gray-100 text-sm" />
      </div>

      <!-- Status badge example -->
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Status</label>
        <div class="py-2">
          {{ if .Item.IsActive }}
          <span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-400">Active</span>
          {{ else }}
          <span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-400">Inactive</span>
          {{ end }}
        </div>
      </div>

      <!-- Edit button at bottom (NO destructive actions here) -->
      <div class="pt-4 mt-4 border-t border-gray-200 dark:border-gray-700">
        <a href="/items/{{ .Item.ID }}/edit"
           class="px-3 py-1 bg-indigo-600 text-white text-sm rounded hover:bg-indigo-700">
          Edit Item
        </a>
      </div>
    </div>
  </div>
</div>
{{ end }}
```

**Key Points:**
- Read-only display only - no Revoke/Delete buttons
- Single "Edit" button at bottom inside content box
- Use `bg-gray-50 dark:bg-gray-700` for readonly inputs

---

## Edit Page Pattern

The edit page has a form with Save/Cancel, plus Danger Zone sections for destructive actions.

```html
{{ define "content" }}
<div class="flex flex-col h-full">
  <!-- Header with back button -->
  <div class="mb-4 flex items-center">
    <a href="/items/{{ .ID }}" class="text-sm px-3 py-1 border dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 mr-2 no-loader">← Back</a>
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">✏️ Edit Item</h1>
  </div>

  <!-- Main content box -->
  <div class="p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4">
    {{ if .Error }}
    <div class="mb-4 p-2 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 rounded max-w-xl">
      {{ .Error }}
    </div>
    {{ end }}

    <!-- Edit form -->
    <form method="POST" action="/items/{{ .ID }}/edit" class="space-y-3 max-w-xl">
      <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">

      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name *</label>
        <input name="name" type="text" value="{{ .Name }}" required
               class="w-full border dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 p-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
      </div>

      <div class="flex gap-2 pt-2">
        <button type="submit" class="bg-indigo-600 text-white px-3 py-1 rounded hover:bg-indigo-700 text-sm">Save Changes</button>
        <a href="/items/{{ .ID }}" class="px-3 py-1 border dark:border-gray-600 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">Cancel</a>
      </div>
    </form>

    <!-- Optional: Info note (blue) -->
    <div class="max-w-xl mt-4 p-4 bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded">
      <h3 class="text-sm font-medium text-blue-800 dark:text-blue-300 mb-1">Note</h3>
      <p class="text-sm text-blue-700 dark:text-blue-400">Additional info for the user...</p>
    </div>

    <!-- Optional: Warning action (amber) - e.g., Revoke -->
    {{ if .CanRevoke }}
    <div class="max-w-xl mt-4">
      <div class="p-4 border border-amber-300 dark:border-amber-700 rounded bg-amber-50 dark:bg-amber-950">
        <h3 class="text-sm font-semibold text-amber-800 dark:text-amber-300 mb-2">Revoke Item</h3>
        <p class="text-xs text-amber-700 dark:text-amber-400 mb-3">Revoking disables this but keeps history.</p>
        <form method="post" action="/items/{{ .ID }}/revoke">
          <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
          <button type="submit" class="bg-amber-600 text-white px-3 py-1 rounded hover:bg-amber-700 text-sm"
                  onclick="return confirm('Are you sure?');">
            Revoke Item
          </button>
        </form>
      </div>
    </div>
    {{ end }}

    <!-- Danger Zone (red) - Delete action -->
    <div class="max-w-xl mt-4">
      <div class="p-4 border border-red-300 dark:border-red-700 rounded bg-red-50 dark:bg-red-900/20">
        <h3 class="text-sm font-semibold text-red-800 dark:text-red-300 mb-2">Danger Zone</h3>
        <p class="text-xs text-red-700 dark:text-red-400 mb-3">Permanently delete this item. This cannot be undone.</p>
        <form method="post" action="/items/{{ .ID }}/delete">
          <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
          <button type="submit" class="bg-red-600 text-white px-3 py-1 rounded hover:bg-red-700 text-sm"
                  onclick="return confirm('Are you sure you want to permanently delete this?');">
            Delete Item
          </button>
        </form>
      </div>
    </div>
  </div>
</div>
{{ end }}
```

**Key Points:**
- Form has Save/Cancel buttons
- Danger Zone at bottom with red styling
- Optional amber section for "soft" destructive actions (revoke, disable)
- All sections use `max-w-xl` for consistent width

---

## New/Create Page Pattern

Similar to Edit but simpler - no Danger Zone needed.

```html
{{ define "content" }}
<div class="flex flex-col h-full">
  <div class="mb-4 flex items-center">
    <a href="/items" class="text-sm px-3 py-1 border dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 mr-2 no-loader">← Back</a>
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">🔑 Create Item</h1>
  </div>

  <div class="p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4">
    {{ if .Error }}
    <div class="mb-4 p-2 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 rounded max-w-xl">{{ .Error }}</div>
    {{ end }}

    <form method="POST" action="/items" class="space-y-3 max-w-xl">
      <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">

      <!-- Form fields -->

      <div class="flex gap-2 pt-2">
        <button type="submit" class="bg-indigo-600 text-white px-3 py-1 rounded hover:bg-indigo-700 text-sm">Create Item</button>
        <a href="/items" class="px-3 py-1 border dark:border-gray-600 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">Cancel</a>
      </div>
    </form>

    <!-- Optional: Warning note (amber) -->
    <div class="max-w-xl mt-4 p-4 bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-800 rounded">
      <h3 class="text-sm font-medium text-amber-800 dark:text-amber-300 mb-1">Important</h3>
      <p class="text-sm text-amber-700 dark:text-amber-400">Warning message...</p>
    </div>
  </div>
</div>
{{ end }}
```

---

## Color Reference

### Styled Boxes

| Type | Border | Background | Title | Text |
|------|--------|------------|-------|------|
| **Info (blue)** | `border-blue-200 dark:border-blue-800` | `bg-blue-50 dark:bg-blue-950` | `text-blue-800 dark:text-blue-300` | `text-blue-700 dark:text-blue-400` |
| **Warning (amber)** | `border-amber-300 dark:border-amber-700` | `bg-amber-50 dark:bg-amber-950` | `text-amber-800 dark:text-amber-300` | `text-amber-700 dark:text-amber-400` |
| **Danger (red)** | `border-red-300 dark:border-red-700` | `bg-red-50 dark:bg-red-900/20` | `text-red-800 dark:text-red-300` | `text-red-700 dark:text-red-400` |
| **Success (green)** | `border-green-200 dark:border-green-800` | `bg-green-50 dark:bg-green-900/20` | `text-green-800 dark:text-green-400` | `text-green-700 dark:text-green-300` |

### Buttons

| Type | Classes |
|------|---------|
| **Primary** | `bg-indigo-600 text-white px-3 py-1 rounded text-sm hover:bg-indigo-700` |
| **Secondary/Cancel** | `px-3 py-1 border dark:border-gray-600 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700` |
| **Danger** | `bg-red-600 text-white px-3 py-1 rounded text-sm hover:bg-red-700` |
| **Warning** | `bg-amber-600 text-white px-3 py-1 rounded text-sm hover:bg-amber-700` |

### Status Badges

| Status | Classes |
|--------|---------|
| **Active/Success** | `px-2 py-1 rounded-full text-xs bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-400` |
| **Inactive/Disabled** | `px-2 py-1 rounded-full text-xs bg-gray-200 text-gray-700 dark:bg-gray-600 dark:text-gray-300` |
| **Error/Revoked** | `px-2 py-1 rounded-full text-xs bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-400` |
| **Pending** | `px-2 py-1 rounded-full text-xs bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-400` |

---

## Form Fields

### Text Input

```html
<div>
  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Label</label>
  <input name="field" type="text" value="{{ .Value }}"
         class="w-full border dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 p-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
</div>
```

### Read-only Input

```html
<input type="text" value="{{ .Value }}" readonly
       class="w-full border dark:border-gray-600 p-2 rounded bg-gray-50 dark:bg-gray-700 dark:text-gray-100 text-sm" />
```

### Textarea

```html
<textarea name="field" rows="3"
          class="w-full border dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 p-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400">{{ .Value }}</textarea>
```

### Select

```html
<select name="field"
        class="w-full border dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded p-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400">
  <option value="a" {{ if eq .Value "a" }}selected{{ end }}>Option A</option>
</select>
```

---

## Handler View Model Pattern

When creating view models for these pages:

```go
// types.go
type ItemFormVM struct {
    viewdata.BaseVM
    ID          string
    Name        string
    Description string
    IsEdit      bool   // true for edit page
    IsActive    bool   // for conditional UI (e.g., show Revoke only if active)
    Error       string
}

// handler.go
func (h *Handler) ServeEdit(w http.ResponseWriter, r *http.Request) {
    // ... fetch item ...

    base := viewdata.NewBaseVM(r, h.DB, "Edit Item", "/items/"+idStr)
    data := ItemFormVM{
        BaseVM:   base,
        ID:       item.ID.Hex(),
        Name:     item.Name,
        IsEdit:   true,
        IsActive: item.Status == "active",  // Pass status for template conditionals
    }
    templates.Render(w, r, "items/edit", data)
}
```

---

## Checklist for New Features

When implementing a new feature's UI:

**All Pages:**
- [ ] Use `flex flex-col h-full` as outer container (NOT `max-w-2xl mx-auto`)
- [ ] Use `flex-1 mb-4` on main content box
- [ ] Support both light and dark modes (include `dark:` variants)

**List Pages:**
- [ ] Header (title + button) is OUTSIDE the content box
- [ ] Content box has `p-4` padding (creates space around table)
- [ ] Include `<div id="modal-root"></div>` for manage modals

**Detail/View Pages:**
- [ ] Only "Edit" button at bottom, no destructive actions
- [ ] Include `no-loader` class on back button

**Edit Pages:**
- [ ] Save/Cancel buttons + Danger Zone at bottom
- [ ] Use `max-w-xl` for form/content width inside the box
- [ ] Include CSRF token in all forms
- [ ] Use `onclick="return confirm(...)"` for destructive actions
- [ ] Pass `IsActive` or similar status flag for conditional UI elements
