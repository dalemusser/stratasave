# Button Color Guide

This document defines the color conventions for buttons throughout the StrataHub application.

## Principles

1. **Semantic meaning over visual variety** - Colors communicate intent, not arbitrary categories
2. **Restraint** - Limited palette reduces cognitive load and maintains professional appearance
3. **Consistency** - Same color always means the same type of action

## Color Palette

| Color | Tailwind Class | Semantic Meaning | When to Use |
|-------|----------------|------------------|-------------|
| **Indigo** | `bg-indigo-600` | Primary/affirmative in-app actions | Default for most buttons |
| **Green** | `bg-green-600` | Access external content | Actions that go outside the app |
| **Red** | `bg-red-600` | Destructive/permanent actions | Cannot be undone |
| **Gray outline** | `border` + `text-gray-700` | Secondary/escape actions | Non-primary, reversible |

## Detailed Usage

### Indigo (Primary Actions)

Use for all standard in-app operations:

- **Create**: Add, New, Create
- **Update**: Save, Update, Submit
- **Navigate**: View, Edit, Manage, Users, Resources
- **Confirm**: Done, Confirm, OK
- **Upload**: Upload, Upload CSV, Upload & Preview

```html
<button class="bg-indigo-600 text-white hover:bg-indigo-700">Add Member</button>
```

### Green (External Access)

Use when the action takes the user outside the normal app flow:

- **Launch**: Open URL in new tab, Launch resource
- **Download**: Download file, Export

```html
<a href="..." target="_blank" class="bg-green-600 text-white hover:bg-green-700">Launch</a>
<a href="..." download class="bg-green-600 text-white hover:bg-green-700">Download</a>
```

### Red (Destructive Actions)

Use only for permanent, irreversible actions:

- **Delete**: Delete record, Remove permanently
- **Revoke**: Revoke access (if permanent)

Always pair with a confirmation dialog.

```html
<button class="bg-red-600 text-white hover:bg-red-700"
        onclick="return confirm('Are you sure?')">Delete</button>
```

### Gray Outline (Secondary Actions)

Use for non-primary actions that don't advance the user's goal:

- **Cancel**: Cancel, Close modal
- **Navigate back**: Back, Return
- **Reset**: Clear search, Reset form
- **Dismiss**: Close, Dismiss

```html
<button class="border text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700">Cancel</button>
```

## What NOT to Do

### Don't use color for arbitrary categories

Wrong approach:
- Blue for Edit
- Green for View
- Purple for CSV
- Orange for Reports

This creates learned arbitrariness where users must memorize color meanings that don't help them.

### Don't overuse colors

A rainbow of button colors looks unprofessional and increases cognitive load. If you're tempted to add a 5th color, reconsider whether it's truly a distinct semantic category.

## Examples

### Form Actions
```
[Save] (indigo)  [Cancel] (gray outline)
```

### List Row Actions
```
[View] (indigo)  [Edit] (indigo)  [Delete] (red)
```

### Resource Actions
```
[Launch] (green)  [Edit] (indigo)  [Delete] (red)
```

### Modal Actions
```
[Confirm] (indigo)  [Cancel] (gray outline)
```

### File Actions
```
[Download] (green)  [Replace] (indigo)  [Remove] (red)
```

## Dark Mode

All button colors have dark mode variants:

| Light | Dark Hover |
|-------|------------|
| `hover:bg-indigo-700` | Same |
| `hover:bg-green-700` | Same |
| `hover:bg-red-700` | Same |
| `hover:bg-gray-50` | `dark:hover:bg-gray-700` |

Gray outline buttons need explicit dark mode classes:
```html
class="border text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
```
