package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/harshRZP/job-scheduler/internal/api"
	"github.com/harshRZP/job-scheduler/internal/api/handler"
	"github.com/harshRZP/job-scheduler/internal/api/middleware"
	"github.com/harshRZP/job-scheduler/internal/config"
	"github.com/harshRZP/job-scheduler/internal/executor"
	"github.com/harshRZP/job-scheduler/internal/repository"
	"github.com/harshRZP/job-scheduler/internal/scheduler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := openDB(cfg.DBDsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Repositories
	jobRepo := repository.NewMySQLJobRepository(db)
	runRepo := repository.NewMySQLRunRepository(db)

	// Executor
	exec := executor.NewHTTPExecutor(cfg.HTTPTimeoutSec)

	// Scheduler
	sched := scheduler.NewMinHeapScheduler(exec, runRepo)
	notifier := scheduler.NewInProcessNotifier(sched)

	// Auth
	auth := middleware.NewStaticKeyAuthenticator(cfg.APIKey)

	// Handlers
	jobH := handler.NewJobHandler(jobRepo, notifier)
	runH := handler.NewRunHandler(runRepo)

	// Load active jobs into the scheduler before accepting traffic
	ctx := context.Background()
	if err := loadActiveJobs(ctx, jobRepo, sched); err != nil {
		log.Fatalf("load active jobs: %v", err)
	}

	if err := sched.Start(); err != nil {
		log.Fatalf("start scheduler: %v", err)
	}
	log.Println("scheduler started")

	// HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      api.NewServer(jobH, runH, auth),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background, wait for shutdown signal
	go func() {
		log.Printf("server listening on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	sched.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}

	log.Println("stopped")
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}

func loadActiveJobs(ctx context.Context, jobRepo repository.JobRepository, sched scheduler.Scheduler) error {
	jobs, err := jobRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active jobs: %w", err)
	}
	for _, j := range jobs {
		if err := sched.AddJob(j); err != nil {
			log.Printf("warn: could not schedule job %s (%s): %v", j.ID, j.Name, err)
		}
	}
	log.Printf("loaded %d active jobs into scheduler", len(jobs))
	return nil
}
