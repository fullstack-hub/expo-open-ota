package handlers

import (
	"expo-open-ota/internal/auth"
	"expo-open-ota/internal/dashboard"
	"net/http"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	dashboardEnabled := dashboard.IsDashboardEnabled()
	if !dashboardEnabled {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	password := r.FormValue("password")
	if password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	authService := auth.NewAuth()
	authResponse, err := authService.LoginWithPassword(password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"token":"` + authResponse.Token + `","refreshToken":"` + authResponse.RefreshToken + `"}`))
	w.WriteHeader(http.StatusOK)
}

func RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	dashboardEnabled := dashboard.IsDashboardEnabled()
	if !dashboardEnabled {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	refreshToken := r.FormValue("refreshToken")
	if refreshToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	authService := auth.NewAuth()
	authResponse, err := authService.RefreshToken(refreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"token":"` + authResponse.Token + `","refreshToken":"` + authResponse.RefreshToken + `"}`))
	w.WriteHeader(http.StatusOK)
}
