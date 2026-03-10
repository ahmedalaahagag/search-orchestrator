package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ahmedalaahagag/search-orchestrator/internal/api"
	"github.com/ahmedalaahagag/search-orchestrator/internal/infra/observability"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/opensearch"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/qus"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/orchestrator"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newHTTPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http",
		Short: "Start the HTTP server",
		RunE:  runHTTP,
	}
}

func runHTTP(cmd *cobra.Command, args []string) error {
	logger := observability.NewLogger()

	cfg, err := config.Load("SEARCH")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"port":       cfg.HTTP.Port,
		"opensearch": cfg.OpenSearch.URL,
		"qus":        cfg.QUS.URL,
	}).Info("configuration loaded")

	metrics := observability.NewMetrics()

	searchCfg, err := config.LoadSearchConfig(filepath.Join(cfg.ConfigDir, "search.yaml"))
	if err != nil {
		return fmt.Errorf("loading search config: %w", err)
	}

	qusClient := qus.NewClient(qus.ClientConfig{
		BaseURL: cfg.QUS.URL,
		Timeout: cfg.QUS.Timeout,
	})

	osClient := opensearch.NewClient(opensearch.ClientConfig{
		URL:      cfg.OpenSearch.URL,
		Username: cfg.OpenSearch.Username,
		Password: cfg.OpenSearch.Password,
		Timeout:  cfg.OpenSearch.Timeout,
	})

	planner := orchestrator.NewPlanner(searchCfg)
	orch := orchestrator.New(logger, metrics, qusClient, osClient, planner, searchCfg)

	router := api.NewRouter(logger, orch, metrics)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.WithField("port", cfg.HTTP.Port).Info("starting HTTP server")
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.WithField("signal", sig.String()).Info("shutting down")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
