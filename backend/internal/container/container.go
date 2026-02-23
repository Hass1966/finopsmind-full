// Package container provides dependency injection.
package container

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/finopsmind/backend/internal/config"
	"github.com/finopsmind/backend/internal/jobs"
	"github.com/finopsmind/backend/internal/mlclient"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/provider"
	"github.com/finopsmind/backend/internal/provider/aws"
	"github.com/finopsmind/backend/internal/provider/azure"
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

	// Initialize ML client
	c.mlClient = mlclient.NewClient(cfg.MLSidecar)
	logger.Info("ML client initialized", "url", cfg.MLSidecar.URL, "enabled", cfg.MLSidecar.Enabled)

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

func (c *Container) Config() *config.Config             { return c.cfg }
func (c *Container) Logger() *slog.Logger               { return c.logger }
func (c *Container) DB() *sql.DB                        { return c.db }
func (c *Container) MLClient() *mlclient.Client         { return c.mlClient }
func (c *Container) ProviderRegistry() *provider.Registry { return c.providerRegistry }
func (c *Container) CostRepository() repository.CostRepository { return c.costRepo }
func (c *Container) BudgetRepository() repository.BudgetRepository { return c.budgetRepo }
func (c *Container) AnomalyRepository() repository.AnomalyRepository { return c.anomalyRepo }
func (c *Container) OrganizationRepository() repository.OrganizationRepository { return c.organizationRepo }
func (c *Container) UserRepository() repository.UserRepository { return c.userRepo }
func (c *Container) RemediationRepository() repository.RemediationRepository { return c.remediationRepo }
func (c *Container) CloudProviderRepository() repository.CloudProviderRepository { return c.cloudProviderRepo }

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
	// Implementation would call ML sidecar with historical cost data
	return nil
}

func (c *Container) forecastJob(ctx context.Context) error {
	c.logger.Info("running forecast job")
	// Implementation would call ML sidecar to generate forecasts
	return nil
}

func (c *Container) budgetCheckJob(ctx context.Context) error {
	c.logger.Info("running budget check job")
	// Implementation would check all budgets against current spend
	return nil
}
