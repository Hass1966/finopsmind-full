// Package container provides dependency injection.
package container

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/finopsmind/backend/internal/config"
	"github.com/finopsmind/backend/internal/jobs"
	"github.com/finopsmind/backend/internal/mlclient"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/notification"
	"github.com/finopsmind/backend/internal/provider"
	"github.com/finopsmind/backend/internal/provider/aws"
	"github.com/finopsmind/backend/internal/provider/azure"
	"github.com/finopsmind/backend/internal/recommendations"
	"github.com/finopsmind/backend/internal/recommendations/rules"
	"github.com/finopsmind/backend/internal/repository"
)

// Container holds all application dependencies.
type Container struct {
	cfg              *config.Config
	logger           *slog.Logger
	db               *sql.DB
	mlClient         *mlclient.Client
	providerRegistry *provider.Registry
	scheduler        *jobs.Scheduler

	// Repositories
	costRepo           repository.CostRepository
	budgetRepo         repository.BudgetRepository
	anomalyRepo        repository.AnomalyRepository
	recommendationRepo repository.RecommendationRepository
	organizationRepo   repository.OrganizationRepository
	userRepo           repository.UserRepository
	remediationRepo    repository.RemediationRepository
	cloudProviderRepo  repository.CloudProviderRepository
	forecastRepo       repository.ForecastRepository

	// Services
	recEngine    *recommendations.Engine
	notifService *notification.Service

	encryptionKey string
}

// New creates a new dependency container.
func New(cfg *config.Config, logger *slog.Logger) (*Container, error) {
	c := &Container{
		cfg:    cfg,
		logger: logger,
	}

	// Initialize database
	db, err := sql.Open("pgx", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.MaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db
	logger.Info("database connected", "host", cfg.Database.Host, "database", cfg.Database.Name)

	// Initialize repositories
	c.costRepo = repository.NewPostgresCostRepository(db)
	c.budgetRepo = repository.NewPostgresBudgetRepository(db)
	c.anomalyRepo = repository.NewPostgresAnomalyRepository(db)
	c.organizationRepo = repository.NewPostgresOrganizationRepository(db)
	c.userRepo = repository.NewPostgresUserRepository(db)
	c.remediationRepo = repository.NewPostgresRemediationRepository(db)
	c.cloudProviderRepo = repository.NewPostgresCloudProviderRepository(db)
	c.encryptionKey = cfg.EncryptionKey

	// Initialize forecast repository and ensure table exists
	forecastRepo := repository.NewPostgresForecastRepository(db)
	if err := forecastRepo.EnsureTable(ctx); err != nil {
		logger.Warn("failed to ensure forecasts table", "error", err)
	}
	c.forecastRepo = forecastRepo

	// Initialize ML client
	c.mlClient = mlclient.NewClient(cfg.MLSidecar)
	logger.Info("ML client initialized", "url", cfg.MLSidecar.URL, "enabled", cfg.MLSidecar.Enabled)

	// Initialize recommendation engine with all rules
	c.recEngine = recommendations.NewEngine(&dbAdapter{db: db}, nil, nil)
	rules.RegisterAllRules(c.recEngine)
	logger.Info("recommendation engine initialized", "rules", len(c.recEngine.GetRules()))

	// Initialize notification service
	var webhookURLs []string
	if cfg.Notification.WebhookURLs != "" {
		webhookURLs = strings.Split(cfg.Notification.WebhookURLs, ",")
	}
	c.notifService = notification.NewService(notification.Config{
		SlackWebhookURL: cfg.Notification.SlackWebhookURL,
		EmailSMTPHost:   cfg.Notification.EmailSMTPHost,
		EmailSMTPPort:   cfg.Notification.EmailSMTPPort,
		EmailFrom:       cfg.Notification.EmailFrom,
		EmailPassword:   cfg.Notification.EmailPassword,
		WebhookURLs:     webhookURLs,
	}, logger)
	logger.Info("notification service initialized")

	// Initialize provider registry
	c.providerRegistry = provider.NewRegistry()

	// Register AWS provider if enabled
	if cfg.AWS.Enabled {
		awsProvider, err := aws.NewProvider(cfg.AWS, logger)
		if err != nil {
			logger.Warn("failed to initialize AWS provider", "error", err)
		} else {
			c.providerRegistry.Register("aws", awsProvider)
			logger.Info("AWS provider registered", "region", cfg.AWS.Region)
		}
	}

	// Register Azure provider if enabled
	if cfg.Azure.Enabled {
		azureProvider, err := azure.NewProvider(cfg.Azure, logger)
		if err != nil {
			logger.Warn("failed to initialize Azure provider", "error", err)
		} else {
			c.providerRegistry.Register("azure", azureProvider)
			logger.Info("Azure provider registered", "subscription", cfg.Azure.SubscriptionID)
		}
	}

	// Initialize scheduler
	c.scheduler = jobs.NewScheduler(logger)

	return c, nil
}

// Start starts background jobs.
func (c *Container) Start(ctx context.Context) error {
	// Register jobs
	c.scheduler.Register("cost-sync", c.cfg.Jobs.CostSyncSchedule, c.costSyncJob)
	c.scheduler.Register("anomaly-detect", c.cfg.Jobs.AnomalyDetectSchedule, c.anomalyDetectJob)
	c.scheduler.Register("forecast", c.cfg.Jobs.ForecastSchedule, c.forecastJob)
	c.scheduler.Register("budget-check", c.cfg.Jobs.BudgetCheckSchedule, c.budgetCheckJob)
	c.scheduler.Register("recommendations", c.cfg.Jobs.RecommendationSchedule, c.recommendationsJob)

	// Start scheduler
	return c.scheduler.Start()
}

// Stop gracefully stops all components.
func (c *Container) Stop(ctx context.Context) error {
	c.logger.Info("stopping container components")

	// Stop scheduler
	if c.scheduler != nil {
		c.scheduler.Stop()
	}

	// Close provider registry
	if c.providerRegistry != nil {
		c.providerRegistry.Close()
	}

	// Close database
	if c.db != nil {
		c.db.Close()
	}

	return nil
}

// Accessors

func (c *Container) Config() *config.Config                                      { return c.cfg }
func (c *Container) Logger() *slog.Logger                                        { return c.logger }
func (c *Container) DB() *sql.DB                                                 { return c.db }
func (c *Container) MLClient() *mlclient.Client                                  { return c.mlClient }
func (c *Container) ProviderRegistry() *provider.Registry                        { return c.providerRegistry }
func (c *Container) CostRepository() repository.CostRepository                   { return c.costRepo }
func (c *Container) BudgetRepository() repository.BudgetRepository               { return c.budgetRepo }
func (c *Container) AnomalyRepository() repository.AnomalyRepository             { return c.anomalyRepo }
func (c *Container) OrganizationRepository() repository.OrganizationRepository    { return c.organizationRepo }
func (c *Container) UserRepository() repository.UserRepository                   { return c.userRepo }
func (c *Container) RemediationRepository() repository.RemediationRepository      { return c.remediationRepo }
func (c *Container) CloudProviderRepository() repository.CloudProviderRepository  { return c.cloudProviderRepo }
func (c *Container) ForecastRepository() repository.ForecastRepository            { return c.forecastRepo }
func (c *Container) RecommendationEngine() *recommendations.Engine               { return c.recEngine }
func (c *Container) NotificationService() *notification.Service                   { return c.notifService }
func (c *Container) DBQuerier() recommendations.DBQuerier                         { return &dbAdapter{db: c.db} }

// Background job implementations

func (c *Container) costSyncJob(ctx context.Context) error {
	c.logger.Info("running cost sync job")

	enabledProviders, err := c.cloudProviderRepo.GetAllEnabled(ctx)
	if err != nil {
		c.logger.Error("failed to get enabled providers", "error", err)
		return err
	}

	if len(enabledProviders) == 0 {
		c.logger.Info("no enabled providers configured, skipping cost sync")
		return nil
	}

	for _, cp := range enabledProviders {
		c.logger.Info("syncing costs from provider", "provider", cp.ProviderType, "org", cp.OrganizationID)

		prov, err := provider.NewProviderFromEncryptedCreds(string(cp.ProviderType), cp.Credentials, c.encryptionKey, c.logger)
		if err != nil {
			c.logger.Error("failed to instantiate provider", "provider", cp.ProviderType, "error", err)
			c.cloudProviderRepo.UpdateStatus(ctx, cp.ID, "error", err.Error())
			continue
		}

		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)

		costs, err := prov.GetCosts(ctx, provider.CostRequest{
			StartDate:   startDate,
			EndDate:     endDate,
			Granularity: "daily",
			GroupBy:     []string{"service"},
		})
		prov.Close()

		if err != nil {
			c.logger.Error("failed to fetch costs", "provider", cp.ProviderType, "error", err)
			c.cloudProviderRepo.UpdateStatus(ctx, cp.ID, "error", "sync failed: "+err.Error())
			continue
		}

		// Convert to CostRecords and persist
		var records []*model.CostRecord
		for _, item := range costs.Costs {
			records = append(records, &model.CostRecord{
				BaseEntity:     model.NewBaseEntity(),
				OrganizationID: cp.OrganizationID,
				Date:           item.Date,
				Amount:         item.Amount,
				Currency:       model.CurrencyUSD,
				Provider:       cp.ProviderType,
				Service:        item.Service,
				AccountID:      item.AccountID,
				Region:         item.Region,
			})
		}

		if len(records) > 0 {
			if err := c.costRepo.CreateBatch(ctx, records); err != nil {
				c.logger.Error("failed to persist costs", "provider", cp.ProviderType, "error", err)
				c.cloudProviderRepo.UpdateStatus(ctx, cp.ID, "error", "persist failed: "+err.Error())
				continue
			}
		}

		c.cloudProviderRepo.UpdateStatus(ctx, cp.ID, "connected", "sync completed")
		c.cloudProviderRepo.UpdateLastSync(ctx, cp.ID)
		c.logger.Info("cost sync completed", "provider", cp.ProviderType, "records", len(records), "total", costs.TotalAmount)
	}

	return nil
}

func (c *Container) anomalyDetectJob(ctx context.Context) error {
	c.logger.Info("running anomaly detection job")

	// Get all organizations with enabled providers
	orgs, err := c.organizationRepo.List(ctx)
	if err != nil {
		c.logger.Error("failed to list organizations", "error", err)
		return err
	}

	for _, org := range orgs {
		// Fetch last 30 days of daily aggregated cost data
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)

		rows, err := c.db.QueryContext(ctx, `
			SELECT date, SUM(amount) as daily_total
			FROM costs
			WHERE organization_id = $1 AND date >= $2 AND date <= $3
			GROUP BY date
			ORDER BY date
		`, org.ID, startDate, endDate)
		if err != nil {
			c.logger.Error("failed to query costs for anomaly detection", "org", org.ID, "error", err)
			continue
		}

		var dataPoints []mlclient.CostDataPoint
		for rows.Next() {
			var date time.Time
			var amount float64
			if err := rows.Scan(&date, &amount); err != nil {
				continue
			}
			dataPoints = append(dataPoints, mlclient.CostDataPoint{
				Date:   date.Format("2006-01-02"),
				Amount: amount,
			})
		}
		rows.Close()

		if len(dataPoints) < 7 {
			c.logger.Info("insufficient data for anomaly detection", "org", org.ID, "points", len(dataPoints))
			continue
		}

		// Call ML sidecar
		result, err := c.mlClient.DetectAnomalies(ctx, &mlclient.AnomalyDetectionRequest{
			OrganizationID: org.ID.String(),
			Data:           dataPoints,
			Sensitivity:    0.1,
		})
		if err != nil {
			c.logger.Error("ML anomaly detection failed", "org", org.ID, "error", err)
			continue
		}

		c.logger.Info("anomaly detection completed", "org", org.ID, "anomalies", result.AnomalyCount)

		// Persist detected anomalies
		for _, a := range result.Anomalies {
			date, _ := time.Parse("2006-01-02", a.Date)
			severity := model.ClassifyAnomalySeverity(a.DeviationPct)

			anomaly := &model.Anomaly{
				BaseEntity:     model.NewBaseEntity(),
				OrganizationID: org.ID,
				Date:           date,
				ActualAmount:   a.ActualAmount,
				ExpectedAmount: a.ExpectedAmount,
				Deviation:      a.Deviation,
				DeviationPct:   a.DeviationPct,
				Score:          a.Score,
				Severity:       severity,
				Status:         model.StatusOpen,
				DetectedAt:     time.Now().UTC(),
			}

			if err := c.anomalyRepo.Create(ctx, anomaly); err != nil {
				c.logger.Error("failed to persist anomaly", "org", org.ID, "error", err)
				continue
			}

			// Send notification for high/critical anomalies
			if severity == model.SeverityHigh || severity == model.SeverityCritical {
				notifSvc := c.getOrgNotificationService(ctx, org)
				if notifSvc != nil {
					if err := notifSvc.SendAnomalyAlert(ctx, "Multiple Services", "multi-cloud", string(severity), a.DeviationPct, a.ActualAmount, a.ExpectedAmount); err != nil {
						c.logger.Error("failed to send anomaly alert", "org", org.ID, "error", err)
					}
				}
			}
		}
	}

	return nil
}

func (c *Container) forecastJob(ctx context.Context) error {
	c.logger.Info("running forecast job")

	orgs, err := c.organizationRepo.List(ctx)
	if err != nil {
		c.logger.Error("failed to list organizations", "error", err)
		return err
	}

	for _, org := range orgs {
		// Fetch last 90 days of daily cost data
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -90)

		rows, err := c.db.QueryContext(ctx, `
			SELECT date, SUM(amount) as daily_total
			FROM costs
			WHERE organization_id = $1 AND date >= $2 AND date <= $3
			GROUP BY date
			ORDER BY date
		`, org.ID, startDate, endDate)
		if err != nil {
			c.logger.Error("failed to query costs for forecast", "org", org.ID, "error", err)
			continue
		}

		var dataPoints []mlclient.CostDataPoint
		for rows.Next() {
			var date time.Time
			var amount float64
			if err := rows.Scan(&date, &amount); err != nil {
				continue
			}
			dataPoints = append(dataPoints, mlclient.CostDataPoint{
				Date:   date.Format("2006-01-02"),
				Amount: amount,
			})
		}
		rows.Close()

		if len(dataPoints) < 14 {
			c.logger.Info("insufficient data for forecast", "org", org.ID, "points", len(dataPoints))
			continue
		}

		// Call ML sidecar
		result, err := c.mlClient.Forecast(ctx, &mlclient.ForecastRequest{
			OrganizationID: org.ID.String(),
			HistoricalDays: len(dataPoints),
			ForecastDays:   30,
			Granularity:    "daily",
			Data:           dataPoints,
		})
		if err != nil {
			c.logger.Error("ML forecast failed", "org", org.ID, "error", err)
			continue
		}

		c.logger.Info("forecast generated", "org", org.ID, "total_forecasted", result.TotalForecasted)

		// Convert ML response to model and persist
		var predictions []model.ForecastPoint
		for _, f := range result.Forecasts {
			date, _ := time.Parse("2006-01-02", f.Date.Format("2006-01-02"))
			predictions = append(predictions, model.ForecastPoint{
				Date:       date,
				Predicted:  f.Predicted,
				LowerBound: f.LowerBound,
				UpperBound: f.UpperBound,
			})
		}

		forecast := &model.Forecast{
			BaseEntity:      model.NewBaseEntity(),
			OrganizationID:  org.ID,
			GeneratedAt:     result.GeneratedAt,
			ModelVersion:    result.ModelVersion,
			Granularity:     model.GranularityDaily,
			Predictions:     predictions,
			TotalForecasted: result.TotalForecasted,
			ConfidenceLevel: result.ConfidenceLevel,
			Currency:        model.CurrencyUSD,
		}

		if err := c.forecastRepo.Create(ctx, forecast); err != nil {
			c.logger.Error("failed to persist forecast", "org", org.ID, "error", err)
		}
	}

	return nil
}

func (c *Container) budgetCheckJob(ctx context.Context) error {
	c.logger.Info("running budget check job")

	orgs, err := c.organizationRepo.List(ctx)
	if err != nil {
		c.logger.Error("failed to list organizations", "error", err)
		return err
	}

	for _, org := range orgs {
		budgets, err := c.budgetRepo.List(ctx, org.ID)
		if err != nil {
			c.logger.Error("failed to list budgets", "org", org.ID, "error", err)
			continue
		}

		for _, budget := range budgets {
			if budget.Status == model.BudgetStatusInactive {
				continue
			}

			// Calculate current spend from costs table for budget period
			periodStart, periodEnd := getBudgetPeriod(budget.Period)

			var currentSpend float64
			err := c.db.QueryRowContext(ctx, `
				SELECT COALESCE(SUM(amount), 0)
				FROM costs
				WHERE organization_id = $1 AND date >= $2 AND date <= $3
			`, org.ID, periodStart, periodEnd).Scan(&currentSpend)
			if err != nil {
				c.logger.Error("failed to calculate budget spend", "budget", budget.ID, "error", err)
				continue
			}

			// Update budget spend
			if err := c.budgetRepo.UpdateSpend(ctx, budget.ID, currentSpend, 0); err != nil {
				c.logger.Error("failed to update budget spend", "budget", budget.ID, "error", err)
				continue
			}

			// Check thresholds
			spendPct := currentSpend / budget.Amount
			oldStatus := budget.Status

			if spendPct >= 1.0 {
				budget.Status = model.BudgetStatusExceeded
			} else if spendPct >= 0.8 {
				budget.Status = model.BudgetStatusWarning
			} else {
				budget.Status = model.BudgetStatusActive
			}

			// Update status if changed
			if budget.Status != oldStatus {
				budget.CurrentSpend = currentSpend
				if err := c.budgetRepo.Update(ctx, budget); err != nil {
					c.logger.Error("failed to update budget status", "budget", budget.ID, "error", err)
				}
			}

			// Send notifications for threshold crossings
			exceeded := spendPct >= 1.0
			notifSvc := c.getOrgNotificationService(ctx, org)

			for i := range budget.Thresholds {
				threshold := &budget.Thresholds[i]
				thresholdPct := threshold.Percentage / 100.0

				if spendPct >= thresholdPct && !threshold.Triggered {
					threshold.Triggered = true
					now := time.Now().UTC()
					threshold.TriggeredAt = &now

					if notifSvc != nil {
						if err := notifSvc.SendBudgetAlert(ctx, budget.Name, budget.Amount, currentSpend, thresholdPct, exceeded); err != nil {
							c.logger.Error("failed to send budget alert", "budget", budget.ID, "error", err)
						}
					}
				}
			}

			c.logger.Info("budget checked",
				"budget", budget.Name,
				"spend", currentSpend,
				"amount", budget.Amount,
				"pct", fmt.Sprintf("%.1f%%", spendPct*100),
				"status", budget.Status,
			)
		}
	}

	return nil
}

func (c *Container) recommendationsJob(ctx context.Context) error {
	c.logger.Info("running recommendations job")

	enabledProviders, err := c.cloudProviderRepo.GetAllEnabled(ctx)
	if err != nil {
		c.logger.Error("failed to get enabled providers", "error", err)
		return err
	}

	for _, cp := range enabledProviders {
		c.logger.Info("fetching recommendations from provider", "provider", cp.ProviderType, "org", cp.OrganizationID)

		prov, err := provider.NewProviderFromEncryptedCreds(string(cp.ProviderType), cp.Credentials, c.encryptionKey, c.logger)
		if err != nil {
			c.logger.Error("failed to instantiate provider for recommendations", "provider", cp.ProviderType, "error", err)
			continue
		}

		recs, err := prov.GetRecommendations(ctx, provider.RecommendationRequest{})
		prov.Close()

		if err != nil {
			c.logger.Error("failed to fetch recommendations", "provider", cp.ProviderType, "error", err)
			continue
		}

		// Save recommendations via engine
		var engineRecs []recommendations.Recommendation
		for _, r := range recs.Recommendations {
			engineRecs = append(engineRecs, recommendations.Recommendation{
				ID:                recommendations.GenerateRecommendationID(string(r.Type), r.ResourceID),
				Type:              recommendations.RecommendationType(r.Type),
				RuleID:            string(r.Type),
				ResourceID:        r.ResourceID,
				ResourceType:      r.ResourceType,
				AccountID:         r.AccountID,
				Region:            r.Region,
				CurrentState:      r.CurrentConfig,
				RecommendedAction: r.RecommendedConfig,
				EstimatedSavings:  r.EstimatedSavings,
				Confidence:        recommendations.ConfidenceHigh,
				Severity:          recommendations.CalculateSeverity(r.EstimatedSavings),
			})
		}

		if len(engineRecs) > 0 {
			if err := c.recEngine.SaveRecommendations(ctx, engineRecs); err != nil {
				c.logger.Error("failed to save recommendations", "provider", cp.ProviderType, "error", err)
			}
		}

		c.logger.Info("recommendations synced", "provider", cp.ProviderType, "count", len(engineRecs), "total_savings", recs.TotalSavings)

		// Send notification for high-savings recommendations
		notifSvc := c.getOrgNotificationServiceByID(ctx, cp.OrganizationID)
		if notifSvc != nil {
			for _, r := range engineRecs {
				if r.EstimatedSavings >= 100 {
					if err := notifSvc.SendRecommendationAlert(ctx, r.ResourceType, string(r.Type), r.EstimatedSavings); err != nil {
						c.logger.Error("failed to send recommendation alert", "error", err)
					}
				}
			}
		}
	}

	return nil
}

// getOrgNotificationService returns a notification service configured for the org's preferences.
func (c *Container) getOrgNotificationService(ctx context.Context, org *model.Organization) *notification.Service {
	// Use org-specific Slack webhook if configured, else fall back to global
	slackURL := c.cfg.Notification.SlackWebhookURL
	if org.Settings.SlackWebhookURL != "" {
		slackURL = org.Settings.SlackWebhookURL
	}

	emailFrom := c.cfg.Notification.EmailFrom
	if !org.Settings.AlertsEnabled {
		return nil
	}

	var webhookURLs []string
	if c.cfg.Notification.WebhookURLs != "" {
		webhookURLs = strings.Split(c.cfg.Notification.WebhookURLs, ",")
	}

	return notification.NewService(notification.Config{
		SlackWebhookURL: slackURL,
		EmailSMTPHost:   c.cfg.Notification.EmailSMTPHost,
		EmailSMTPPort:   c.cfg.Notification.EmailSMTPPort,
		EmailFrom:       emailFrom,
		EmailPassword:   c.cfg.Notification.EmailPassword,
		EmailRecipients: org.Settings.EmailRecipients,
		WebhookURLs:     webhookURLs,
	}, c.logger)
}

func (c *Container) getOrgNotificationServiceByID(ctx context.Context, orgID interface{}) *notification.Service {
	// Try to get org from repo
	if id, ok := orgID.(interface{ String() string }); ok {
		_ = id // org notification falls back to global
	}
	return c.notifService
}

// getBudgetPeriod returns the start and end dates for a budget period.
func getBudgetPeriod(period model.BudgetPeriod) (time.Time, time.Time) {
	now := time.Now().UTC()
	switch period {
	case model.BudgetPeriodMonthly:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		return start, end
	case model.BudgetPeriodQuarterly:
		quarter := (now.Month() - 1) / 3
		start := time.Date(now.Year(), quarter*3+1, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 3, -1)
		return start, end
	case model.BudgetPeriodYearly:
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, time.UTC)
		return start, end
	default:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		return start, end
	}
}

// dbAdapter wraps sql.DB to implement the recommendations.DBQuerier interface.
type dbAdapter struct {
	db *sql.DB
}

func (a *dbAdapter) Query(query string, args ...interface{}) (recommendations.Rows, error) {
	return a.db.Query(query, args...)
}

func (a *dbAdapter) QueryRow(query string, args ...interface{}) recommendations.Row {
	return a.db.QueryRow(query, args...)
}
