package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sagnikc395/anchora"
	"github.com/sagnikc395/anchora/config"
	"github.com/sagnikc395/anchora/httpapi"
	"github.com/sagnikc395/anchora/huggingfaceagent"
	"github.com/sagnikc395/anchora/jobs"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the YAML configuration file")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	agents := make(httpapi.AgentRegistry, len(cfg.Agents))
	for name, definition := range cfg.Agents {
		agent, err := huggingfaceagent.New(ctx, huggingfaceagent.Config{Name: name, ModelID: definition.ModelID, TokenEnv: definition.TokenEnv, Instruction: definition.Instruction, MaxTokens: definition.MaxTokens, Timeout: definition.Timeout()})
		if err != nil {
			log.Fatalf("configure agent %q: %v", name, err)
		}
		agents[name] = agent
	}

	options := httpapi.Options{MaxRetries: cfg.Workflow.MaxRetries, RetryDelay: cfg.Workflow.RetryDelay()}
	var service *jobs.Service
	var workers sync.WaitGroup
	if cfg.Async.Enabled {
		store, err := jobs.NewStore(ctx, cfg.Async.DatabaseURL())
		if err != nil {
			log.Fatalf("PostgreSQL: %v", err)
		}
		defer store.Close()
		queue, err := jobs.NewQueue(ctx, cfg.Async.RedisURL(), cfg.Async.QueueName)
		if err != nil {
			log.Fatalf("Redis: %v", err)
		}
		defer queue.Close()

		service = &jobs.Service{
			Store:   store,
			Queue:   queue,
			Agents:  agents,
			Options: anchora.Options{MaxRetries: options.MaxRetries, RetryDelay: options.RetryDelay},
			Config: jobs.Config{
				Lease:          cfg.Async.Lease(),
				Heartbeat:      cfg.Async.Heartbeat(),
				MaxAttempts:    cfg.Async.MaxAttempts,
				ReaperInterval: cfg.Async.ReaperInterval(),
				WorkerTTL:      cfg.Async.WorkerTTL(),
				ReclaimBatch:   cfg.Async.ReclaimBatch,
				QueueName:      cfg.Async.QueueName,
			},
			Logf: log.Printf,
		}

		for i := 0; i < cfg.Async.WorkerCount(); i++ {
			workerID, err := jobs.NewWorkerID()
			if err != nil {
				log.Fatalf("generate worker ID: %v", err)
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				if err := service.RunWorker(ctx, workerID); err != nil && ctx.Err() == nil {
					log.Printf("worker %s stopped: %v", workerID, err)
				}
			}()
		}
		if cfg.Async.ReaperEnabled() {
			workers.Add(1)
			go func() {
				defer workers.Done()
				if err := service.RunReaper(ctx); err != nil && ctx.Err() == nil {
					log.Printf("reaper stopped: %v", err)
				}
			}()
		}
		log.Printf("async enabled: %d worker(s), %s lease, reaper=%t", cfg.Async.WorkerCount(), cfg.Async.Lease(), cfg.Async.ReaperEnabled())
	}

	server := &http.Server{Addr: cfg.Server.Address, Handler: httpapi.NewRouterWithJobs(agents, options, service)}
	go func() {
		log.Printf("anchora listening on %s", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	// Stop accepting work, then give in-flight jobs a moment to release their
	// leases so a rolling restart does not wait out the visibility timeout.
	log.Print("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-shutdown.Done():
		log.Print("workers did not drain in time; their leases will expire")
	}
}
