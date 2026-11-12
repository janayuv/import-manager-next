package models

// ExceptionCase represents an operational import exception tracked in exception_cases.
type ExceptionCase struct {
	ID            string `json:"id"`
	ExceptionType string `json:"exceptionType"` // OVERDUE_ETA | MISSING_BOE | MISSING_EXPENSE
	EntityType    string `json:"entityType"`
	EntityID      string `json:"entityId"`
	Status        string `json:"status"`   // OPEN | IN_PROGRESS | RESOLVED | IGNORED
	Priority      string `json:"priority"` // LOW | MEDIUM | HIGH | CRITICAL
	AssignedTo    string `json:"assignedTo"`
	AssignedAt    string `json:"assignedAt"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	ResolvedBy    string `json:"resolvedBy"`
	ResolvedAt    string `json:"resolvedAt"`
	SLADeadline   string `json:"slaDeadline"`
	SLAStatus     string `json:"slaStatus"` // ON_TIME | BREACHED
}

// ExceptionNote stores investigation notes attached to an exception case.
type ExceptionNote struct {
	NoteID          string `json:"noteId"`
	ExceptionCaseID string `json:"exceptionCaseId"`
	UserID          string `json:"userId"`
	NoteText        string `json:"noteText"`
	CreatedAt       string `json:"createdAt"`
}

// ExceptionDetectionResult summarizes one exception scan & SLA pass.
type ExceptionDetectionResult struct {
	CreatedCount    int `json:"createdCount"`
	UpdatedCount    int `json:"updatedCount"`
	SelfHealedCount int `json:"selfHealedCount"`
	EscalatedCount  int `json:"escalatedCount"`
	ActiveOpenCount int `json:"activeOpenCount"`
}

// WorkflowRule represents an automation decision rule in workflow_decision_rules.
type WorkflowRule struct {
	RuleID              string `json:"ruleId"`
	RuleName            string `json:"ruleName"`
	RuleType            string `json:"ruleType"`
	ConditionExpression string `json:"conditionExpression"`
	ActionType          string `json:"actionType"`
	Priority            int    `json:"priority"`
	Enabled             bool   `json:"enabled"`
	TenantID            string `json:"tenantId"`
	LatestVersionID     string `json:"latestVersionId,omitempty"`
	LatestVersionNum    int    `json:"latestVersionNum,omitempty"`
	CurrentEnv          string `json:"currentEnv,omitempty"`
	CanaryPercent       float64 `json:"canaryPercent,omitempty"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

// SaveWorkflowRuleInput creates or updates a workflow rule and records a version snapshot.
type SaveWorkflowRuleInput struct {
	RuleID              string `json:"ruleId,omitempty"`
	RuleName            string `json:"ruleName"`
	RuleType            string `json:"ruleType"`
	ConditionExpression string `json:"conditionExpression"`
	ActionType          string `json:"actionType"`
	Priority            int    `json:"priority"`
	Enabled             bool   `json:"enabled"`
	ChangeReason        string `json:"changeReason"`
}

// RuleRiskAssessment scores deployment blast-radius risk prior to PROD rollout.
type RuleRiskAssessment struct {
	RuleID        string   `json:"ruleId"`
	VersionID     string   `json:"versionId"`
	Score         float64  `json:"score"`
	Level         string   `json:"level"` // LOW | MEDIUM | HIGH | CRITICAL
	BlocksProd    bool     `json:"blocksProd"`
	Factors       []string `json:"factors"`
}

// CanaryStatus reports FNV-1a deterministic canary rollout configuration for a rule.
type CanaryStatus struct {
	ID                   string  `json:"id"`
	RuleID               string  `json:"ruleId"`
	VersionID            string  `json:"versionId"`
	SampleSizePercentage float64 `json:"sampleSizePercentage"`
	DeploymentStatus     string  `json:"deploymentStatus"`
	CreatedAt            string  `json:"createdAt"`
}

// AutomationCostLimit mirrors automation_cost_limits.
type AutomationCostLimit struct {
	ID                         string  `json:"id"`
	MaxCostUnitsPerCycle       float64 `json:"maxCostUnitsPerCycle"`
	MaxExecutionTimePerCycleMs int64   `json:"maxExecutionTimePerCycleMs"`
	MaxRecordsProcessedCycle   int64   `json:"maxRecordsProcessedPerCycle"`
	UpdatedAt                  string  `json:"updatedAt"`
}

// BackgroundJob represents one scheduled system job in workflow_background_jobs.
type BackgroundJob struct {
	JobID              string `json:"jobId"`
	JobName            string `json:"jobName"`
	JobType            string `json:"jobType"`
	ScheduleType       string `json:"scheduleType"`
	IsEnabled          bool   `json:"isEnabled"`
	ExpectedDurationMs int64  `json:"expectedDurationMs"`
	MaxRetries         int    `json:"maxRetries"`
	RetryDelaySec      int    `json:"retryDelaySec"`
	LastStatus         string `json:"lastStatus,omitempty"`
	LastStartedAt      string `json:"lastStartedAt,omitempty"`
	LastCompletedAt    string `json:"lastCompletedAt,omitempty"`
}

// JobExecutionRecord mirrors workflow_job_execution_log.
type JobExecutionRecord struct {
	ExecutionID      string `json:"executionId"`
	JobID            string `json:"jobId"`
	StartedAt        string `json:"startedAt"`
	CompletedAt      string `json:"completedAt"`
	Status           string `json:"status"`
	RecordsProcessed int    `json:"recordsProcessed"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
	ExecutionTimeMs  int64  `json:"executionTimeMs"`
	RetryCount       int    `json:"retryCount"`
}

// WorkflowIncident mirrors workflow_incidents.
type WorkflowIncident struct {
	IncidentID      string `json:"incidentId"`
	AlertID         string `json:"alertId"`
	IncidentType    string `json:"incidentType"`
	Severity        string `json:"severity"` // FATAL | CRITICAL | HIGH | WARNING
	Status          string `json:"status"`   // OPEN | ACKNOWLEDGED | RESOLVED
	AssignedTo      string `json:"assignedTo"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	ResolvedAt      string `json:"resolvedAt,omitempty"`
	ResolutionNotes string `json:"resolutionNotes,omitempty"`
	EscalationCount int    `json:"escalationCount"`
}

// IncidentNote mirrors workflow_incident_notes.
type IncidentNote struct {
	NoteID     string `json:"noteId"`
	IncidentID string `json:"incidentId"`
	NoteText   string `json:"noteText"`
	CreatedBy  string `json:"createdBy"`
	CreatedAt  string `json:"createdAt"`
}

// ResolveIncidentInput enforces the multi-signal incident resolution gate.
type ResolveIncidentInput struct {
	IncidentID           string `json:"incidentId"`
	ResolutionNotes      string `json:"resolutionNotes"`
	SkipQuietWindowCheck bool   `json:"skipQuietWindowCheck"`
}

// FailureForecast mirrors 30m predictive failure forecasts.
type FailureForecast struct {
	ForecastID           string  `json:"forecastId"`
	SourceModule         string  `json:"sourceModule"`
	EventType            string  `json:"eventType"`
	ForecastTime         string  `json:"forecastTime"`
	PredictedProbability float64 `json:"predictedProbability"`
	ConfidenceScore      float64 `json:"confidenceScore"`
	HorizonMinutes       int     `json:"horizonMinutes"`
	DetailsJSON          string  `json:"detailsJson"`
}

// OperationsOverview summarizes live workflow health, jobs, incidents, bursts, and forecasts.
type OperationsOverview struct {
	WorkflowHealthScore float64            `json:"workflowHealthScore"`
	SystemStability     float64            `json:"systemStability"`
	OpenExceptions      int                `json:"openExceptions"`
	BreachedSLACount    int                `json:"breachedSlaCount"`
	OpenIncidents       int                `json:"openIncidents"`
	ActiveSuppressions  int                `json:"activeSuppressions"`
	RecentBursts        int                `json:"recentBursts"`
	RestoreLocked       bool               `json:"restoreLocked"`
	Jobs                []BackgroundJob    `json:"jobs"`
	Forecasts           []FailureForecast  `json:"forecasts"`
	Incidents           []WorkflowIncident `json:"incidents"`
}

// ErrorMemoryRecord mirrors deduplicated errors in error_memory.
type ErrorMemoryRecord struct {
	ID              string `json:"id"`
	Fingerprint     string `json:"fingerprint"`
	OccurrenceCount int    `json:"occurrenceCount"`
	FirstSeenAt     string `json:"firstSeenAt"`
	LastSeenAt      string `json:"lastSeenAt"`
	ModuleName      string `json:"moduleName"`
	CommandName     string `json:"commandName"`
	ErrorCode       string `json:"errorCode"`
	ErrorMessage    string `json:"errorMessage"`
	Severity        string `json:"severity"`
	Status          string `json:"status"` // new | acknowledged | resolved
	AISummary       string `json:"aiSummary,omitempty"`
}

// RecordErrorInput captures a runtime error for deduplicated persistence in error_memory.
type RecordErrorInput struct {
	ModuleName          string `json:"moduleName"`
	CommandName         string `json:"commandName"`
	PageName            string `json:"pageName,omitempty"`
	ErrorCode           string `json:"errorCode,omitempty"`
	ErrorCategory       string `json:"errorCategory,omitempty"`
	ErrorMessage        string `json:"errorMessage"`
	StackTrace          string `json:"stackTrace,omitempty"`
	Severity            string `json:"severity,omitempty"`
	Recoverable         bool   `json:"recoverable"`
	RedactedContextJSON string `json:"redactedContextJson,omitempty"`
}

// BugNote mirrors developer/operator bug_notes.
type BugNote struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // OPEN | SOLVED
	Context     string `json:"context"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// SaveBugNoteInput creates or updates a bug note.
type SaveBugNoteInput struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status,omitempty"`
	Context     string `json:"context,omitempty"`
}

// DiagnosticsExportResult reports path & summary of the generated diagnostics bundle zip.
type DiagnosticsExportResult struct {
	FilePath         string `json:"filePath"`
	SizeBytes        int64  `json:"sizeBytes"`
	GeneratedAt      string `json:"generatedAt"`
	IncludedSections []string `json:"includedSections"`
}
