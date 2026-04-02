package infrastructure

import (
	"encoding/json"
	"expo-open-ota/config"
	"expo-open-ota/internal/appcontext"
	"expo-open-ota/internal/dashboard"
	"expo-open-ota/internal/handlers"
	"expo-open-ota/internal/metrics"
	"expo-open-ota/internal/middleware"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func getDashboardPath() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)

	if strings.Contains(exePath, "/var/folders/") || strings.Contains(exePath, "Temp") {
		workingDir, _ := os.Getwd()
		return filepath.Join(workingDir, "apps", "dashboard", "dist")
	}
	return filepath.Join(exeDir, "apps", "dashboard", "dist")
}


func registerRoutes(r *mux.Router) {
	r.HandleFunc("/manifest", handlers.ManifestHandler).Methods(http.MethodGet)
	r.HandleFunc("/assets", handlers.AssetsHandler).Methods(http.MethodGet)
	r.HandleFunc("/requestUploadUrl/{BRANCH}", handlers.RequestUploadUrlHandler).Methods(http.MethodPost)
	r.HandleFunc("/uploadLocalFile", handlers.RequestUploadLocalFileHandler).Methods(http.MethodPut)
	r.HandleFunc("/markUpdateAsUploaded/{BRANCH}", handlers.MarkUpdateAsUploadedHandler).Methods(http.MethodPost)
	r.HandleFunc("/rollback/{BRANCH}", handlers.RollbackHandler).Methods(http.MethodPost)
	r.HandleFunc("/republish/{BRANCH}", handlers.RepublishHandler).Methods(http.MethodPost)

	corsSubrouter := r.PathPrefix("/auth").Subrouter()
	corsSubrouter.HandleFunc("/login", handlers.LoginHandler).Methods(http.MethodPost)
	corsSubrouter.HandleFunc("/refreshToken", handlers.RefreshTokenHandler).Methods(http.MethodPost)

	authSubrouter := r.PathPrefix("/api").Subrouter()
	authSubrouter.Use(middleware.AuthMiddleware)
	authSubrouter.HandleFunc("/settings", handlers.GetSettingsHandler).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/branches", handlers.GetBranchesHandler).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/channels", handlers.GetChannelsHandler).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/branch/{BRANCH}/runtimeVersions", handlers.GetRuntimeVersionsHandler).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/branch/{BRANCH}/runtimeVersion/{RUNTIME_VERSION}/updates", handlers.GetUpdatesHandler).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/branch/{BRANCH}/runtimeVersion/{RUNTIME_VERSION}/updates/{UPDATE_ID}", handlers.GetUpdateDetails).Methods(http.MethodGet)
	authSubrouter.HandleFunc("/branch/{BRANCH}/updateChannelBranchMapping", handlers.UpdateChannelBranchMappingHandler).Methods(http.MethodPost)
}

func NewRouter() http.Handler {
	r := mux.NewRouter()
	r.Use(middleware.LoggingMiddleware)

	r.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.PrometheusHandler().ServeHTTP(w, r)
	}).Methods(http.MethodGet)

	r.HandleFunc("/hc", HealthCheck).Methods(http.MethodGet)

	dashboardPath := getDashboardPath()


	// Register dashboard routes BEFORE /{APP_SLUG} to avoid conflict
	if dashboard.IsDashboardEnabled() {
		serveDashboard := func(appSlug string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				basePath := "/dashboard"
				if appSlug != "" {
					basePath = "/" + appSlug + "/dashboard"
				} else if config.IsMultiAppMode() {
					vars := mux.Vars(r)
					if slug := vars["APP_SLUG"]; slug != "" {
						appSlug = slug
						basePath = "/" + slug + "/dashboard"
					}
				}

				if strings.HasSuffix(r.URL.Path, "/env.js") {
					w.Header().Set("Content-Type", "application/javascript")
					baseURL := config.GetEnv("BASE_URL")
					if baseURL == "" {
						baseURL = "http://localhost:3000"
					}
					apiURL := baseURL
					if appSlug != "" {
						apiURL = baseURL + "/" + appSlug
					}
					dashboardBasename := "/dashboard"
						if appSlug != "" {
							dashboardBasename = "/" + appSlug + "/dashboard"
						}
						w.Write([]byte(fmt.Sprintf("window.env = { VITE_OTA_API_URL: '%s', VITE_DASHBOARD_BASENAME: '%s' };", apiURL, dashboardBasename)))
					return
				}
				if r.URL.Path == basePath {
					target := basePath + "/"
					if r.URL.RawQuery != "" {
						target += "?" + r.URL.RawQuery
					}
					http.Redirect(w, r, target, http.StatusMovedPermanently)
					return
				}
				staticExtensions := []string{".css", ".js", ".svg", ".png", ".json", ".ico"}
				for _, ext := range staticExtensions {
					if len(r.URL.Path) > len(ext) && r.URL.Path[len(r.URL.Path)-len(ext):] == ext {
						relPath := strings.TrimPrefix(r.URL.Path, basePath+"/")
						// Also try stripping /dashboard/ for absolute paths from HTML
						if strings.HasPrefix(r.URL.Path, "/dashboard/") && basePath != "/dashboard" {
							relPath = strings.TrimPrefix(r.URL.Path, "/dashboard/")
						}
						filePath := filepath.Join(dashboardPath, relPath)
						if !strings.HasPrefix(filePath, dashboardPath) {
							http.Error(w, "Forbidden", http.StatusForbidden)
							return
						}
						http.ServeFile(w, r, filePath)
						return
					}
				}
				filePath := filepath.Join(dashboardPath, "index.html")
				http.ServeFile(w, r, filePath)
			}
		}

		if config.IsMultiAppMode() {
			// Serve dashboard under /{appSlug}/dashboard for each app
			for _, app := range config.GetAllApps() {
				r.PathPrefix("/" + app.Slug + "/dashboard").HandlerFunc(serveDashboard(app.Slug))
			}
			// /dashboard/ static files are handled by middleware above
		} else {
			r.PathPrefix("/dashboard").HandlerFunc(serveDashboard(""))
		}
	}

	if config.IsMultiAppMode() {
		// Multi-app mode: register routes for each configured app explicitly
		for _, app := range config.GetAllApps() {
			appRouter := r.PathPrefix("/" + app.Slug).Subrouter()
			appCopy := app
			appRouter.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := appcontext.WithAppConfig(r.Context(), &appCopy)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			registerRoutes(appRouter)
		}

		// List available apps at root
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			apps := config.GetAllApps()
			slugs := make([]string, len(apps))
			for i, app := range apps {
				slugs[i] = app.Slug
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"mode":"multi-app","apps":%s}`, mustJSON(slugs))
		}).Methods(http.MethodGet)
	} else {
		// Single-app mode: routes at root (backward compatible)
		registerRoutes(r)
	}

	// Wrap router with dashboard static file handler for multi-app mode
	if dashboard.IsDashboardEnabled() && config.IsMultiAppMode() {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/dashboard/") || req.URL.Path == "/dashboard" {
				// env.js is dynamically generated
				if strings.HasSuffix(req.URL.Path, "/env.js") {
					apps := config.GetAllApps()
					slug := ""
					if len(apps) > 0 {
						slug = apps[0].Slug
					}
					w.Header().Set("Content-Type", "application/javascript")
					baseURL := config.GetEnv("BASE_URL")
					apiURL := baseURL
					if slug != "" {
						apiURL = baseURL + "/" + slug
					}
					dashboardBasename := "/dashboard"
						if slug != "" {
							dashboardBasename = "/" + slug + "/dashboard"
						}
						w.Write([]byte(fmt.Sprintf("window.env = { VITE_OTA_API_URL: '%s', VITE_DASHBOARD_BASENAME: '%s' };", apiURL, dashboardBasename)))
					return
				}
				// Static files
				staticExtensions := []string{".css", ".js", ".svg", ".png", ".json", ".ico"}
				for _, ext := range staticExtensions {
					if strings.HasSuffix(req.URL.Path, ext) {
						relPath := strings.TrimPrefix(req.URL.Path, "/dashboard/")
						filePath := filepath.Join(dashboardPath, relPath)
						if strings.HasPrefix(filePath, dashboardPath) {
							http.ServeFile(w, req, filePath)
							return
						}
					}
				}
				// Redirect /dashboard to first app dashboard
				if req.URL.Path == "/dashboard" || req.URL.Path == "/dashboard/" {
					apps := config.GetAllApps()
					if len(apps) > 0 {
						http.Redirect(w, req, "/"+apps[0].Slug+"/dashboard/", http.StatusFound)
						return
					}
				}
			}
			r.ServeHTTP(w, req)
		})
	}

	return r
}

func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}
