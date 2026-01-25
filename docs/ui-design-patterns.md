# UI Design Patterns

This document describes the standard UI design patterns used in StrataSave. Follow these patterns consistently across all features.

## Table of Contents

1. [Page Structure](#page-structure)
2. [Page Headers](#page-headers)
3. [Two-Pane Layouts](#two-pane-layouts)
4. [Content Containers](#content-containers)
5. [Tables](#tables)
6. [Badges and Pills](#badges-and-pills)
7. [Buttons](#buttons)
8. [Manage Modal](#manage-modal)
9. [Danger Zone](#danger-zone)
10. [Forms](#forms)
11. [Profile-Style Sections](#profile-style-sections)
12. [Messages](#messages)
13. [Color Reference](#color-reference)
14. [Pagination](#pagination)

---

## Page Structure

All pages use a flex column layout that fills the available height:

```html
{{ define "content" }}
<div class="flex flex-col h-full">
  <!-- Page Header -->
  <div class="mb-4 flex items-center justify-between">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Page Title</h1>
  </div>

  <!-- Main Content -->
  <div class="p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4">
    <!-- Content here -->
  </div>
</div>
{{ end }}
```

**Key classes:**
- Outer wrapper: `flex flex-col h-full` - fills available height
- Content container: `flex-1 mb-4` - grows to fill space, margin before footer

---

## Page Headers

### Standard Header (Title + Primary Action)

```html
<div class="mb-4 flex items-center justify-between">
  <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Page Title</h1>
  <a href="/feature/new" class="px-3 py-1 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700">
    Add Item
  </a>
</div>
```

### Header with Icon

Page titles should include an emoji icon for visual identification:

```html
<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">👥 System Users</h1>
<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">📢 Announcements</h1>
<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">✉️ Invitations</h1>
<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">📋 Audit Log</h1>
<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">👤 Profile</h1>
```

### Header with Back Button

For detail/edit pages that navigate away from a list:

```html
<div class="mb-4 flex items-center">
  <a href="{{ .BackURL }}"
     class="text-sm px-3 py-1 border dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 mr-2 no-loader"
     title="Go back">
    ← Back
  </a>
  <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">✏️ Edit System User</h1>
</div>
```

---

## Two-Pane Layouts

Some pages have a two-pane layout with a sidebar (e.g., Organizations filter) and a main content area (e.g., Groups table). These layouts require special handling to align header elements with the panes below.

### Body Structure

The body uses flexbox with a fixed-width sidebar and flexible main section:

```html
<div class="flex-1 flex items-start gap-2 md:gap-4">
  <!-- Sidebar -->
  <aside class="flex-none" style="flex-basis: clamp(220px, 22vw, 280px);">
    <!-- Sidebar content -->
  </aside>

  <!-- Main Section -->
  <section class="flex-1 min-w-[560px] flex flex-col">
    <!-- Search form, table, etc. -->
  </section>
</div>
```

### Header with Toggle Button

When a toggle button (e.g., "Hide Orgs") needs to align with the main section below, the header must mirror the body's flex structure exactly:

```html
<!-- Header - mirrors body flex structure so button aligns with section below -->
{{ if .ShowSidebar }}
<div class="mb-4 flex items-center gap-2 md:gap-4">
  <!-- Title container - matches sidebar width -->
  <div class="flex-none" style="flex-basis: clamp(220px, 22vw, 280px);">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">👥 Groups</h1>
  </div>
  <!-- Actions container - matches main section -->
  <div class="flex-1 flex items-center justify-between">
    <button id="toggle-sidebar"
            class="text-xs px-2 py-1 border dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300"
            type="button">
      Hide Orgs
    </button>
    <a href="/items/new" class="px-3 py-1 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700">
      Add Item
    </a>
  </div>
</div>
{{ else }}
<!-- Standard header when sidebar is hidden -->
<div class="mb-4 flex items-center justify-between">
  <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">👥 Groups</h1>
  <a href="/items/new" class="px-3 py-1 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700">
    Add Item
  </a>
</div>
{{ end }}
```

**Key points:**
- Use the same `gap-2 md:gap-4` in both header and body
- Use the same `flex-basis: clamp(220px, 22vw, 280px)` for the title container and sidebar
- The toggle button goes at the start of the `flex-1` container (left side via `justify-between`)
- The primary action button goes at the end of the `flex-1` container (right side)
- This ensures the toggle button's left edge aligns with the main section's left edge below

**Why this works:** By mirroring the exact flex structure (fixed-width first item + flexible second item + same gap), the toggle button starts at the same horizontal position as the main section content below it.

---

## Content Containers

### Standard Container

Used for most page content (tables, forms, etc.):

```html
<div class="p-4 bg-white dark:bg-gray-800 rounded shadow text-gray-700 dark:text-gray-300 text-sm flex-1 mb-4">
  <!-- Content -->
</div>
```

### Scrollable Container

For tables or content that may overflow:

```html
<div class="p-4 bg-white dark:bg-gray-800 rounded shadow flex-1 mb-4 overflow-auto">
  <table>...</table>
</div>
```

---

## Tables

### Table Structure

```html
<table class="min-w-full text-sm text-left text-gray-700 dark:text-gray-300">
  <thead class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 uppercase text-xs">
    <tr class="border-b border-gray-300 dark:border-gray-600">
      <th class="px-2 py-2">Column Name</th>
      <th class="px-2 py-2 text-right">Actions</th>
    </tr>
  </thead>
  <tbody>
    {{ range .Items }}
    <tr class="border-b border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-900/50">
      <td class="px-2 py-2 align-middle">{{ .Value }}</td>
      <td class="px-2 py-2 align-middle text-right">
        <!-- Manage button -->
      </td>
    </tr>
    {{ else }}
    <tr>
      <td colspan="2" class="px-2 py-6 text-center text-gray-500 dark:text-gray-400">
        No items found.
      </td>
    </tr>
    {{ end }}
  </tbody>
</table>
```

**Header row:**
- Background: `bg-gray-100 dark:bg-gray-700`
- Text: `text-gray-600 dark:text-gray-400 uppercase text-xs`
- Border: `border-b border-gray-300 dark:border-gray-600`

**Body rows:**
- Border: `border-b border-gray-200 dark:border-gray-600`
- Hover: `hover:bg-gray-50 dark:hover:bg-gray-900/50`
- Cell alignment: `align-middle`

---

## Badges and Pills

### Role/Category Pills (Fully Rounded)

```html
<!-- Purple for roles -->
<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-400">
  admin
</span>

<!-- Gray for neutral categories -->
<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-300">
  password
</span>
```

### Status Badges (Slightly Rounded)

```html
<!-- Active/Success -->
<span class="px-2 py-1 text-xs bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400 rounded">Active</span>

<!-- Inactive/Disabled -->
<span class="px-2 py-1 text-xs bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 rounded">Disabled</span>

<!-- Pending -->
<span class="px-2 py-1 text-xs bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400 rounded">Pending</span>

<!-- Expired/Error -->
<span class="px-2 py-1 text-xs bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400 rounded">Expired</span>
```

### Type Badges (Info/Warning/Critical)

```html
<!-- Info (blue) -->
<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400">info</span>

<!-- Warning (yellow) -->
<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400">warning</span>

<!-- Critical (red) -->
<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400">critical</span>
```

---

## Buttons

### Primary Button

```html
<button class="bg-indigo-600 text-white px-3 py-1 rounded text-sm hover:bg-indigo-700">
  Save
</button>
```

### Secondary/Cancel Button

```html
<button class="px-3 py-1 border dark:border-gray-600 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">
  Cancel
</button>
```

### Danger Button

```html
<button class="px-3 py-1 bg-red-600 text-white rounded text-sm hover:bg-red-700">
  Delete
</button>
```

### Manage Button (in tables)

```html
<form
  method="get"
  action="/feature/{{ .ID }}/manage_modal"
  hx-get="/feature/{{ .ID }}/manage_modal?return={{ $.CurrentPath | urlquery }}"
  hx-target="#modal-root"
  hx-swap="innerHTML"
>
  <button
    type="submit"
    class="bg-indigo-600 text-white px-2 py-1 rounded text-xs hover:bg-indigo-700"
    title="Manage item"
  >
    Manage
  </button>
</form>
```

---

## Manage Modal

The Manage modal provides quick access to View, Edit, and Delete actions. Delete is separated into a Danger Zone.

```html
{{ define "feature/manage_modal" }}
<div class="fixed inset-0 z-50 flex items-center justify-center">
  <!-- Backdrop -->
  <div class="absolute inset-0 bg-black/40"
       onclick="document.getElementById('modal-root').innerHTML=''"></div>

  <!-- Modal Content -->
  <div class="relative bg-white dark:bg-gray-800 rounded-xl shadow border border-gray-300 dark:border-gray-600 max-w-md w-full p-4 space-y-4">
    <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Manage Item</h2>

    <p class="text-sm text-gray-700 dark:text-gray-300">
      {{ .Name }} <span class="text-gray-500 dark:text-gray-400">({{ .Identifier }})</span><br/>
      <span class="text-gray-500 dark:text-gray-400">Additional info here</span>
    </p>

    <!-- Primary Actions -->
    <div class="flex justify-center gap-2">
      <a href="/feature/{{ .ID }}" class="px-3 py-1 bg-indigo-600 text-white rounded text-sm hover:bg-indigo-700">View</a>
      <a href="/feature/{{ .ID }}/edit" class="px-3 py-1 bg-indigo-600 text-white rounded text-sm hover:bg-indigo-700">Edit</a>
    </div>

    <!-- Danger Zone -->
    <div class="p-3 border border-red-300 dark:border-red-700 rounded bg-red-50 dark:bg-red-900/20">
      <div class="flex items-center justify-between gap-4">
        <div>
          <div class="text-sm font-semibold text-red-800 dark:text-red-300">Danger Zone</div>
          <p class="text-xs text-red-700 dark:text-red-400">Permanently delete this item.</p>
        </div>
        <form method="post" action="/feature/{{ .ID }}/delete"
              onsubmit="return confirm('Are you sure you want to delete this item?');">
          <input type="hidden" name="return" value="{{ .BackURL }}">
          <button type="submit" class="px-3 py-1 bg-red-600 text-white rounded text-sm hover:bg-red-700">
            Delete
          </button>
        </form>
      </div>
    </div>

    <!-- Close Button -->
    <div class="flex justify-start">
      <button
        type="button"
        class="px-3 py-1 border rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
        onclick="document.getElementById('modal-root').innerHTML=''"
      >
        Close
      </button>
    </div>
  </div>
</div>
{{ end }}
```

**Key elements:**
- Modal root: `<div id="modal-root"></div>` at end of page content
- Backdrop closes modal on click
- View/Edit buttons grouped together
- Danger Zone visually separated with red styling
- Close button at bottom left

---

## Danger Zone

The Danger Zone pattern is used for destructive actions like delete. It appears:
1. In Manage modals (compact horizontal layout)
2. On Edit pages (full-width box at bottom)

### In Modal (Compact)

```html
<div class="p-3 border border-red-300 dark:border-red-700 rounded bg-red-50 dark:bg-red-900/20">
  <div class="flex items-center justify-between gap-4">
    <div>
      <div class="text-sm font-semibold text-red-800 dark:text-red-300">Danger Zone</div>
      <p class="text-xs text-red-700 dark:text-red-400">Permanently delete this item.</p>
    </div>
    <form method="post" action="..." onsubmit="return confirm('...');">
      <button type="submit" class="px-3 py-1 bg-red-600 text-white rounded text-sm hover:bg-red-700">
        Delete
      </button>
    </form>
  </div>
</div>
```

### On Edit Page (Full Width)

Placed at the bottom of the form, separated by a border:

```html
<!-- After form buttons -->
<div class="max-w-xl mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
  <div class="p-4 border border-red-300 dark:border-red-700 rounded bg-red-50 dark:bg-red-900/20">
    <h3 class="text-sm font-semibold text-red-800 dark:text-red-300 mb-2">Danger Zone</h3>
    <p class="text-xs text-red-700 dark:text-red-400 mb-3">
      Permanently delete this user. This action cannot be undone.
    </p>
    <form method="post" action="/feature/{{ .ID }}/delete"
          onsubmit="return confirm('Are you sure you want to delete this item?');">
      <input type="hidden" name="return" value="{{ .BackURL }}">
      <button type="submit" class="bg-red-600 text-white px-3 py-1 rounded text-sm hover:bg-red-700">
        Delete
      </button>
    </form>
  </div>
</div>
```

**Styling:**
- Border: `border border-red-300 dark:border-red-700`
- Background: `bg-red-50 dark:bg-red-900/20`
- Title: `text-red-800 dark:text-red-300`
- Description: `text-red-700 dark:text-red-400`
- Separator above: `border-t border-gray-200 dark:border-gray-700` with `mt-4 pt-4`

---

## Forms

### Form Container

```html
<form method="post" action="..." class="space-y-3 max-w-xl">
  <!-- Form fields -->

  <div class="flex gap-2 pt-2">
    <button type="submit" class="bg-indigo-600 text-white px-3 py-1 rounded hover:bg-indigo-700 text-sm">
      Save
    </button>
    <a href="{{ .BackURL }}" class="px-3 py-1 border dark:border-gray-600 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">
      Cancel
    </a>
  </div>
</form>
```

### Form Field

```html
<div>
  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Field Label</label>
  <input name="field" type="text" value="{{ .Value }}"
         class="w-full border dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 p-2 rounded text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
</div>
```

### Select Field

```html
<div>
  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Select Label</label>
  <select name="field"
          class="w-full border dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded p-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400">
    <option value="a" {{ if eq .Value "a" }}selected{{ end }}>Option A</option>
    <option value="b" {{ if eq .Value "b" }}selected{{ end }}>Option B</option>
  </select>
</div>
```

### Search/Filter Controls

```html
<form
  hx-get="/feature"
  hx-target="#content"
  hx-swap="innerHTML"
  hx-push-url="true"
  hx-trigger="submit, keyup changed delay:300ms from:#search, change from:#status"
  class="bg-white dark:bg-gray-800 rounded shadow p-3 mb-1 flex flex-wrap items-center gap-2"
>
  <input
    id="search" name="search" type="text"
    value="{{ .SearchQuery }}"
    placeholder="Search..."
    class="px-3 py-2 border rounded flex-1 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100" />

  <select id="status" name="status" class="px-3 py-2 border rounded text-sm dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100">
    <option value="">All</option>
    <option value="active">Active</option>
  </select>

  <a href="..." class="px-4 py-2 border rounded text-sm hover:bg-gray-50 dark:hover:bg-gray-700">Clear</a>
</form>
```

---

## Profile-Style Sections

For pages with multiple distinct sections (like Profile):

```html
<div class="space-y-6">
  <!-- Section 1 -->
  <div class="bg-white dark:bg-gray-800 p-4 rounded border dark:border-gray-700">
    <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-3">Section Title</h2>
    <!-- Section content -->
  </div>

  <!-- Section 2 -->
  <div class="bg-white dark:bg-gray-800 p-4 rounded border dark:border-gray-700">
    <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-3">Another Section</h2>
    <!-- Section content -->
  </div>
</div>
```

---

## Messages

### Success Message

```html
{{ if .Success }}
  <div class="bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 p-2 rounded mb-4">
    {{ .Success }}
  </div>
{{ end }}
```

### Error Message

```html
{{ if .Error }}
  <div class="bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 p-2 rounded mb-4">
    {{ .Error }}
  </div>
{{ end }}
```

### Inline Error (in forms)

```html
<div class="mb-4 p-2 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 rounded max-w-xl">
  {{ .Error }}
</div>
```

---

## Color Reference

| Purpose | Light Mode | Dark Mode |
|---------|-----------|-----------|
| Content container bg | `bg-white` | `bg-gray-800` |
| Content container border | `border-gray-300` | `border-gray-600` |
| Table header bg | `bg-gray-100` | `bg-gray-700` |
| Table header text | `text-gray-600` | `text-gray-400` |
| Table body text | `text-gray-700` | `text-gray-300` |
| Header row border | `border-gray-300` | `border-gray-600` |
| Body row border | `border-gray-200` | `border-gray-600` |
| Row hover | `bg-gray-50` | `bg-gray-900/50` |
| Primary button | `bg-indigo-600` | `bg-indigo-600` |
| Danger button | `bg-red-600` | `bg-red-600` |
| Danger zone bg | `bg-red-50` | `bg-red-900/20` |
| Danger zone border | `border-red-300` | `border-red-700` |
| Danger zone text | `text-red-800` | `text-red-300` |
| Success bg | `bg-green-100` | `bg-green-900/30` |
| Success text | `text-green-700` | `text-green-400` |
| Error bg | `bg-red-100` | `bg-red-900/30` |
| Error text | `text-red-700` | `text-red-400` |

---

## Pagination

### Inside Table Container

```html
{{ if gt .TotalPages 1 }}
<div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 flex justify-center gap-2">
  {{ if gt .Page 1 }}
  <a href="?page={{ .PrevPage }}" class="px-3 py-1 border dark:border-gray-600 rounded text-sm hover:bg-gray-50 dark:hover:bg-gray-700">← Previous</a>
  {{ end }}
  <span class="px-3 py-1 text-sm text-gray-600 dark:text-gray-400">Page {{ .Page }} of {{ .TotalPages }}</span>
  {{ if lt .Page .TotalPages }}
  <a href="?page={{ .NextPage }}" class="px-3 py-1 border dark:border-gray-600 rounded text-sm hover:bg-gray-50 dark:hover:bg-gray-700">Next →</a>
  {{ end }}
</div>
{{ end }}
```

### Above Table (with range info)

```html
<div class="flex items-center justify-between mb-1">
  <div class="text-gray-600 dark:text-gray-400 text-sm">
    {{ if .Total }}{{ .RangeStart }}–{{ .RangeEnd }} of {{ .Total }} shown{{ else }}0 of 0 shown{{ end }}
  </div>
  <div class="flex items-center gap-2">
    {{ if .HasPrev }}
      <a class="inline-flex items-center justify-center h-7 leading-none text-xs px-2 border rounded text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 whitespace-nowrap"
         href="?page={{ .PrevPage }}">Prev</a>
    {{ else }}
      <span class="inline-flex items-center justify-center h-7 leading-none text-xs px-2 border rounded text-gray-400 dark:text-gray-500 whitespace-nowrap">Prev</span>
    {{ end }}
    {{ if .HasNext }}
      <a class="inline-flex items-center justify-center h-7 leading-none text-xs px-2 border rounded text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 whitespace-nowrap"
         href="?page={{ .NextPage }}">Next</a>
    {{ else }}
      <span class="inline-flex items-center justify-center h-7 leading-none text-xs px-2 border rounded text-gray-400 dark:text-gray-500 whitespace-nowrap">Next</span>
    {{ end }}
  </div>
</div>
```
