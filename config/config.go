package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	Server   Server                      `yaml:"server"`
	Workflow Workflow                    `yaml:"workflow"`
	Async    Async                       `yaml:"async"`
	Agents   map[string]HuggingFaceAgent `yaml:"agents"`
}
type Async struct {
	Enabled        bool   `yaml:"enabled"`
	DatabaseURLEnv string `yaml:"database_url_env"`
	RedisURLEnv    string `yaml:"redis_url_env"`
	QueueName      string `yaml:"queue_name"`
	Workers        int    `yaml:"workers"`
	// LeaseMS is the visibility timeout: how long a worker's claim on a job
	// survives without a heartbeat before another worker may take it over.
	LeaseMS int `yaml:"lease_ms"`
	// HeartbeatMS is the lease renewal interval. Zero derives it from LeaseMS.
	HeartbeatMS int `yaml:"heartbeat_ms"`
	// MaxAttempts caps job deliveries before dead-lettering. Zero is unlimited.
	MaxAttempts int `yaml:"max_attempts"`
	// ReaperIntervalMS is how often expired leases are swept.
	ReaperIntervalMS int `yaml:"reaper_interval_ms"`
	// WorkerTTLMS is how long a silent worker stays in the registry. Zero
	// derives it from LeaseMS.
	WorkerTTLMS int `yaml:"worker_ttl_ms"`
	// ReclaimBatch caps how many jobs a single reaper sweep recovers.
	ReclaimBatch int `yaml:"reclaim_batch"`
	// Reaper runs the recovery sweep on this node. Leave it on unless you run
	// a dedicated reaper process; the sweeps are atomic and safe to duplicate.
	Reaper *bool `yaml:"reaper"`
}
type Server struct {
	Address string `yaml:"address"`
}
type Workflow struct {
	MaxRetries   int `yaml:"max_retries"`
	RetryDelayMS int `yaml:"retry_delay_ms"`
}
type HuggingFaceAgent struct {
	ModelID     string `yaml:"model_id"`
	TokenEnv    string `yaml:"token_env"`
	Instruction string `yaml:"instruction"`
	MaxTokens   int    `yaml:"max_tokens"`
	TimeoutMS   int    `yaml:"timeout_ms"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8080"
	}
	if cfg.Workflow.MaxRetries < 0 || cfg.Workflow.RetryDelayMS < 0 {
		return Config{}, fmt.Errorf("workflow retry values must be non-negative")
	}
	if cfg.Async.Workers < 0 {
		return Config{}, fmt.Errorf("async workers must be non-negative")
	}
	if cfg.Async.LeaseMS < 0 || cfg.Async.HeartbeatMS < 0 || cfg.Async.ReaperIntervalMS < 0 || cfg.Async.WorkerTTLMS < 0 {
		return Config{}, fmt.Errorf("async lease, heartbeat, reaper, and worker TTL values must be non-negative")
	}
	if cfg.Async.MaxAttempts < 0 || cfg.Async.ReclaimBatch < 0 {
		return Config{}, fmt.Errorf("async max_attempts and reclaim_batch must be non-negative")
	}
	if cfg.Async.HeartbeatMS > 0 && cfg.Async.LeaseMS > 0 && cfg.Async.HeartbeatMS >= cfg.Async.LeaseMS {
		return Config{}, fmt.Errorf("async heartbeat_ms must be shorter than lease_ms")
	}
	if cfg.Async.Enabled {
		if cfg.Async.DatabaseURL() == "" {
			return Config{}, fmt.Errorf("async is enabled but %s is not set", cfg.Async.databaseEnv())
		}
		if cfg.Async.RedisURL() == "" {
			return Config{}, fmt.Errorf("async is enabled but %s is not set", cfg.Async.redisEnv())
		}
	}
	for name, agent := range cfg.Agents {
		if name == "" || agent.ModelID == "" {
			return Config{}, fmt.Errorf("agents must have a name and model_id")
		}
		if agent.MaxTokens < 0 || agent.TimeoutMS < 0 {
			return Config{}, fmt.Errorf("agent %q values must be non-negative", name)
		}
	}
	return cfg, nil
}
func (a Async) databaseEnv() string {
	if a.DatabaseURLEnv == "" {
		return "DATABASE_URL"
	}
	return a.DatabaseURLEnv
}
func (a Async) redisEnv() string {
	if a.RedisURLEnv == "" {
		return "REDIS_URL"
	}
	return a.RedisURLEnv
}
func (a Async) DatabaseURL() string { return os.Getenv(a.databaseEnv()) }
func (a Async) RedisURL() string    { return os.Getenv(a.redisEnv()) }
func (a Async) WorkerCount() int {
	if a.Workers == 0 {
		return 1
	}
	return a.Workers
}

// Lease is the visibility timeout for a claimed job.
func (a Async) Lease() time.Duration {
	if a.LeaseMS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(a.LeaseMS) * time.Millisecond
}

// Heartbeat is the lease renewal interval, defaulting to a third of the lease.
func (a Async) Heartbeat() time.Duration {
	if a.HeartbeatMS <= 0 {
		return a.Lease() / 3
	}
	return time.Duration(a.HeartbeatMS) * time.Millisecond
}

// ReaperInterval is how often expired leases are swept.
func (a Async) ReaperInterval() time.Duration {
	if a.ReaperIntervalMS <= 0 {
		return 5 * time.Second
	}
	return time.Duration(a.ReaperIntervalMS) * time.Millisecond
}

// WorkerTTL is how long a silent worker remains in the registry.
func (a Async) WorkerTTL() time.Duration {
	if a.WorkerTTLMS <= 0 {
		return 4 * a.Lease()
	}
	return time.Duration(a.WorkerTTLMS) * time.Millisecond
}

// ReaperEnabled reports whether this node runs the recovery sweep. It defaults
// to true so a single-node deployment recovers without extra configuration.
func (a Async) ReaperEnabled() bool          { return a.Reaper == nil || *a.Reaper }
func (w Workflow) RetryDelay() time.Duration { return time.Duration(w.RetryDelayMS) * time.Millisecond }
func (a HuggingFaceAgent) Timeout() time.Duration {
	return time.Duration(a.TimeoutMS) * time.Millisecond
}
