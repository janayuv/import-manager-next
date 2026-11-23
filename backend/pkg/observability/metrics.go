package observability

import (
	"context"
	"sync"
	"time"
)

// DefaultSlowQueryThreshold defines the execution threshold above which a query is flagged as slow.
const DefaultSlowQueryThreshold = 50 * time.Millisecond

// JobMetric holds aggregated runtime statistics for a background job.
type JobMetric struct {
	JobID         string        `json:"jobId"`
	TotalRuns     int64         `json:"totalRuns"`
	SuccessRuns   int64         `json:"successRuns"`
	FailedRuns    int64         `json:"failedRuns"`
	TotalDuration time.Duration `json:"totalDuration"`
	LastDuration  time.Duration `json:"lastDuration"`
	LastRunAt     time.Time     `json:"lastRunAt"`
}

// QueryMetric holds execution statistics for database operations.
type QueryMetric struct {
	Operation     string        `json:"operation"`
	Table         string        `json:"table"`
	ExecutionCount int64        `json:"executionCount"`
	SlowCount     int64         `json:"slowCount"`
	TotalDuration time.Duration `json:"totalDuration"`
	MaxDuration   time.Duration `json:"maxDuration"`
}

// Registry tracks thread-safe in-memory observability metrics.
type Registry struct {
	mu                  sync.RWMutex
	jobs                map[string]*JobMetric
	queries             map[string]*QueryMetric
	slowQueryThreshold  time.Duration
	logger              *Logger
}

// NewRegistry constructs a new MetricsRegistry.
func NewRegistry(logger *Logger) *Registry {
	return &Registry{
		jobs:               make(map[string]*JobMetric),
		queries:            make(map[string]*QueryMetric),
		slowQueryThreshold: DefaultSlowQueryThreshold,
		logger:             logger,
	}
}

// SetSlowQueryThreshold configures the slow query cutoff.
func (r *Registry) SetSlowQueryThreshold(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slowQueryThreshold = d
}

// RecordJobDuration records a background job's run time and outcome.
func (r *Registry) RecordJobDuration(jobID string, duration time.Duration, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	metric, exists := r.jobs[jobID]
	if !exists {
		metric = &JobMetric{JobID: jobID}
		r.jobs[jobID] = metric
	}

	metric.TotalRuns++
	if status == "SUCCESS" || status == "COMPLETED" {
		metric.SuccessRuns++
	} else {
		metric.FailedRuns++
	}
	metric.TotalDuration += duration
	metric.LastDuration = duration
	metric.LastRunAt = time.Now().UTC()
}

// RecordQueryTiming records a database operation's execution time and flags slow queries.
func (r *Registry) RecordQueryTiming(ctx context.Context, operation string, table string, duration time.Duration) {
	key := operation + ":" + table

	r.mu.Lock()
	threshold := r.slowQueryThreshold
	metric, exists := r.queries[key]
	if !exists {
		metric = &QueryMetric{Operation: operation, Table: table}
		r.queries[key] = metric
	}

	metric.ExecutionCount++
	metric.TotalDuration += duration
	if duration > metric.MaxDuration {
		metric.MaxDuration = duration
	}

	isSlow := duration >= threshold
	if isSlow {
		metric.SlowCount++
	}
	r.mu.Unlock()

	if isSlow && r.logger != nil {
		r.logger.Warn(ctx, "slow query detected",
			"operation", operation,
			"table", table,
			"duration_ms", duration.Milliseconds(),
			"threshold_ms", threshold.Milliseconds(),
		)
	}
}

// GetJobMetrics returns a snapshot copy of job execution metrics.
func (r *Registry) GetJobMetrics() map[string]JobMetric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string]JobMetric, len(r.jobs))
	for k, v := range r.jobs {
		snapshot[k] = *v
	}
	return snapshot
}

// GetQueryMetrics returns a snapshot copy of database query execution metrics.
func (r *Registry) GetQueryMetrics() map[string]QueryMetric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string]QueryMetric, len(r.queries))
	for k, v := range r.queries {
		snapshot[k] = *v
	}
	return snapshot
}
