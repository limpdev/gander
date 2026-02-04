package app

import (
	"net/http"
	"time"

	"github.com/limpdev/gander/internal/common"
)

func (a *application) handleThemeChangeRequest(w http.ResponseWriter, r *http.Request) {
	themeKey := r.PathValue("key")

	properties, exists := a.Config.Theme.Presets.Get(themeKey)
	if !exists && themeKey != "default" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if themeKey == "default" {
		properties = &a.Config.Theme.ThemeProperties
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "theme",
		Value:    themeKey,
		Path:     a.Config.Server.BaseURL + "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(2 * 365 * 24 * time.Hour),
	})

	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("X-Scheme", common.Ternary(properties.Light, "light", "dark"))
	w.Write([]byte(properties.CSS))
}
