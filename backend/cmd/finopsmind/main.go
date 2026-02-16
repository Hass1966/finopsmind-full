package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/finopsmind/backend/internal/allocation"
	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/config"
	"github.com/finopsmind/backend/internal/container"
	"github.com/finopsmind/backend/internal/handler"
	"github.com/finopsmind/backend/internal/remediation"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize dependency container
	ctr, err := container.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize container", "error", err)
		os.Exit(1)
	}

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://*.finopsmind.io"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (unauthenticated)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		mlHealth, err := ctr.MLClient().Health(ctx)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","ml_sidecar":"unavailable"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"status":"healthy","ml_sidecar":"%s"}`, mlHealth.Status)))
	})

	// Initialize auth
	jwtMgr, err := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	if err != nil {
		logger.Error("failed to initialize JWT manager", "error", err)
		os.Exit(1)
	}

	// Initialize handlers
	costHandler := handler.NewCostHandler(ctr.CostRepository())
	budgetHandler := handler.NewBudgetHandler(ctr.BudgetRepository())
	anomalyHandler := handler.NewAnomalyDBHandler(ctr.AnomalyRepository(), ctr.MLClient())
	authHandler := auth.NewHandler(jwtMgr, ctr.UserRepository(), ctr.OrganizationRepository())
	exportHandler := handler.NewExportHandler(ctr.CostRepository())
	settingsHandler := handler.NewSettingsHandler(ctr.OrganizationRepository())
	allocationSvc := allocation.NewService(ctr.DB())
	allocationHandler := handler.NewAllocationHandler(allocationSvc)
	reportHandler := handler.NewReportHandler(ctr.CostRepository(), ctr.BudgetRepository(), ctr.AnomalyRepository())
	chatHandler := handler.NewChatHandler(ctr.CostRepository(), ctr.AnomalyRepository(), ctr.BudgetRepository())
	policyHandler := handler.NewPolicyHandler()
	k8sHandler := handler.NewKubernetesHandler()
	unitEconHandler := handler.NewUnitEconomicsHandler()
	commitmentHandler := handler.NewCommitmentHandler()
	driftHandler := handler.NewDriftHandler()
	remediationExecutor := remediation.NewExecutor(ctr.RemediationRepository(), logger)
	remediationHandler := handler.NewRemediationHandler(remediationExecutor)

	// Auth middleware
	requireAuth := auth.Middleware(jwtMgr, ctr.UserRepository())

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
			r.Post("/signup", authHandler.Signup)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(requireAuth)

			// Auth
			r.Get("/auth/me", authHandler.Me)
			r.Post("/auth/api-keys", authHandler.CreateAPIKey)

			// Costs
			r.Get("/costs/summary", costHandler.GetSummary)
			r.Get("/costs/trend", costHandler.GetTrend)
			r.Get("/costs/breakdown", costHandler.GetBreakdown)
			r.Get("/costs/export", exportHandler.ExportCSV)

			// Budgets
			r.Get("/budgets", budgetHandler.List)
			r.Post("/budgets", budgetHandler.Create)
			r.Get("/budgets/{id}", budgetHandler.GetByID)
			r.Put("/budgets/{id}", budgetHandler.Update)
			r.Delete("/budgets/{id}", budgetHandler.Delete)

			// Anomalies
			r.Get("/anomalies", anomalyHandler.List)
			r.Get("/anomalies/summary", anomalyHandler.GetSummary)
			r.Patch("/anomalies/{id}", anomalyHandler.Update)
			r.Post("/anomalies/{id}/acknowledge", anomalyHandler.Acknowledge)
			r.Post("/anomalies/{id}/resolve", anomalyHandler.Resolve)

			// Recommendations (keep existing mock for now, will be wired to DB in Phase B)
			r.Get("/recommendations", handler.GetRecommendations)
			r.Patch("/recommendations/{id}", handler.UpdateRecommendation)

			// Forecasts
			r.Get("/forecasts", handler.GetForecasts)

			// Providers
			r.Get("/providers", func(w http.ResponseWriter, r *http.Request) {
				providers := ctr.ProviderRegistry().HealthAll(r.Context())
				results := make([]map[string]interface{}, 0)
				for name, health := range providers {
					results = append(results, map[string]interface{}{
						"id":      name,
						"name":    name,
						"status":  boolToStatus(health.Healthy),
						"healthy": health.Healthy,
						"message": health.Message,
					})
				}
				// If no providers registered, show AWS as the default
				if len(results) == 0 {
					results = append(results, map[string]interface{}{
						"id": "aws", "name": "AWS", "status": "configured",
						"healthy": !cfg.AWS.Enabled, "message": "AWS provider configured",
					})
				}
				handler.WriteJSON(w, http.StatusOK, results)
			})

			// Settings
			r.Get("/settings", settingsHandler.Get)
			r.Put("/settings", settingsHandler.Update)

			// Cost Allocation
			r.Get("/allocations", allocationHandler.GetAllocations)
			r.Get("/allocations/untagged", allocationHandler.GetUntaggedResources)

			// Reports
			r.Get("/reports/executive-summary", reportHandler.ExecutiveSummary)
			r.Get("/reports/comparison", reportHandler.CostComparison)
			r.Get("/reports/export/csv", reportHandler.ExportReportCSV)
			r.Get("/reports/export/json", reportHandler.ExportReportJSON)

			// Chat
			r.Post("/chat", chatHandler.Chat)

			// Policies
			r.Get("/policies", policyHandler.List)
			r.Post("/policies", policyHandler.Create)
			r.Get("/policies/summary", policyHandler.GetSummary)
			r.Get("/policies/violations", policyHandler.GetViolations)
			r.Get("/policies/{id}", policyHandler.GetByID)

			// Remediation
			r.Get("/remediations", remediationHandler.List)
			r.Post("/remediations", remediationHandler.Propose)
			r.Get("/remediations/summary", remediationHandler.GetSummary)
			r.Get("/remediations/{id}", remediationHandler.GetByID)
			r.Post("/remediations/{id}/approve", remediationHandler.Approve)
			r.Post("/remediations/{id}/reject", remediationHandler.Reject)
			r.Post("/remediations/{id}/cancel", remediationHandler.Cancel)
			r.Post("/remediations/{id}/rollback", remediationHandler.Rollback)
			r.Get("/remediations/rules", remediationHandler.ListRules)
			r.Post("/remediations/rules", remediationHandler.CreateRule)
			r.Put("/remediations/rules/{id}", remediationHandler.UpdateRule)
			r.Delete("/remediations/rules/{id}", remediationHandler.DeleteRule)

			// Kubernetes
			r.Get("/kubernetes/clusters", k8sHandler.GetClusters)
			r.Get("/kubernetes/namespaces", k8sHandler.GetNamespaces)
			r.Get("/kubernetes/rightsizing", k8sHandler.GetRightsizing)

			// Unit Economics
			r.Get("/unit-economics", unitEconHandler.GetMetrics)

			// Commitments
			r.Get("/commitments/portfolio", commitmentHandler.GetPortfolio)

			// Drift Detection
			r.Get("/drift/summary", driftHandler.GetSummary)

			// ML management
			r.Route("/ml", func(r chi.Router) {
				r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					health, err := ctr.MLClient().Health(ctx)
					if err != nil {
						http.Error(w, err.Error(), http.StatusServiceUnavailable)
						return
					}
					handler.WriteJSON(w, http.StatusOK, health)
				})
			})
		})
	})

	// Start background jobs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ctr.Start(ctx); err != nil {
		logger.Error("failed to start background jobs", "error", err)
	}

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down server...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer shutdownCancel()

		if err := ctr.Stop(shutdownCtx); err != nil {
			logger.Error("container shutdown error", "error", err)
		}

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	// Start server
	logger.Info("FinOpsMind API server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}

func boolToStatus(healthy bool) string {
	if healthy {
		return "connected"
	}
	return "disconnected"
}
