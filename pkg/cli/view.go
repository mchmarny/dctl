package cli

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mchmarny/devpulse/pkg/data"
)

func faviconHandler() http.HandlerFunc {
	ico, readErr := embedFS.ReadFile("assets/img/favicon.ico")

	return func(w http.ResponseWriter, r *http.Request) {
		if readErr != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, err := w.Write(ico); err != nil {
			slog.Error("failed to write favicon", "error", err)
		}
	}
}

func homeViewHandler(tmpl *template.Template, basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		d := map[string]any{
			"version":       version,
			"commit":        commit,
			"build_date":    date,
			"err":           r.URL.Query().Get("err"),
			"period_months": data.EventAgeMonthsDefault,
			"base_path":     basePath,
		}
		if err := tmpl.ExecuteTemplate(w, "home", d); err != nil {
			slog.Error("template render failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
