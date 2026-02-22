package cli

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mchmarny/devpulse/pkg/data"
)

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	file, err := embedFS.ReadFile("assets/img/favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	if _, err = w.Write(file); err != nil {
		slog.Error("failed to write favicon", "error", err)
	}
}

func homeViewHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d := map[string]any{
			"version":       version,
			"commit":        commit,
			"build_date":    date,
			"err":           r.URL.Query().Get("err"),
			"period_months": data.EventAgeMonthsDefault,
		}
		if err := tmpl.ExecuteTemplate(w, "home", d); err != nil {
			slog.Error("template render failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
