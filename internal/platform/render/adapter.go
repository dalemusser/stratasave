// internal/platform/render/adapter.go
package render

import (
	"net/http"

	"go.uber.org/zap"
)

var engine *Engine

// UseEngine installs the engine used by the helper Render functions.
func UseEngine(e *Engine) { engine = e }

// Render executes a full page (entry template that calls layout).
func Render(w http.ResponseWriter, r *http.Request, name string, data any) {
	if engine == nil {
		zap.L().Error("render called before engine installed", zap.String("name", name))
		http.Error(w, "template exec error", http.StatusInternalServerError)
		return
	}
	if err := engine.Render(w, r, name, data); err != nil {
		zap.L().Error("template render failed", zap.String("name", name), zap.Error(err))
		http.Error(w, "template exec error", http.StatusInternalServerError)
	}
}

// RenderSnippet executes a partial by name (e.g., "groups_table").
func RenderSnippet(w http.ResponseWriter, name string, data any) {
	if engine == nil {
		zap.L().Error("render snippet called before engine installed", zap.String("name", name))
		http.Error(w, "template exec error", http.StatusInternalServerError)
		return
	}
	if err := engine.RenderSnippet(w, name, data); err != nil {
		zap.L().Error("snippet render failed", zap.String("name", name), zap.Error(err))
		http.Error(w, "template exec error", http.StatusInternalServerError)
	}
}

// RenderAutoMap picks a snippet based on HX-Target; if HX-Target is "content",
// it renders the page's content-only block. Otherwise it renders the full page.
// Example:
//
//	render.RenderAutoMap(w, r, "groups_list", map[string]string{
//	    "groups-table-wrap": "groups_table",
//	}, data)
func RenderAutoMap(w http.ResponseWriter, r *http.Request, page string, targets map[string]string, data any) {
	if engine == nil {
		zap.L().Error("render auto called before engine installed", zap.String("page", page))
		http.Error(w, "template exec error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		hxTarget := r.Header.Get("HX-Target")
		// First, explicit target->snippet mapping
		if snip, ok := targets[hxTarget]; ok && snip != "" {
			if err := engine.RenderSnippet(w, snip, data); err != nil {
				zap.L().Error("snippet render failed", zap.String("snippet", snip), zap.Error(err))
				http.Error(w, "template exec error", http.StatusInternalServerError)
			}
			return
		}
		// Fallback: if the target is the main page body, render just the content block
		if hxTarget == "content" {
			if err := engine.RenderContent(w, page, data); err != nil {
				zap.L().Error("content render failed", zap.String("page", page), zap.Error(err))
				http.Error(w, "template exec error", http.StatusInternalServerError)
			}
			return
		}
	}

	// Not HTMX (or no special mapping) → full page with layout
	if err := engine.Render(w, r, page, data); err != nil {
		zap.L().Error("template render failed", zap.String("page", page), zap.Error(err))
		http.Error(w, "template exec error", http.StatusInternalServerError)
	}
}

// Convenience for the common single-table swap case:
// render.RenderAuto(w, r, "admin_organizations_list", "admin_organizations_table", "orgs-table-wrap", data)
func RenderAuto(w http.ResponseWriter, r *http.Request, page, tableSnippet, targetID string, data any) {
	RenderAutoMap(w, r, page, map[string]string{targetID: tableSnippet}, data)
}
