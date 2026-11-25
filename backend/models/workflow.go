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
	RuleID               string  `json:"ruleId"`
	RuleName             string  `json:"ruleName"`
	RuleType             string  `json:"ruleType"`
	ConditionExpression  string  `json:"conditionExpression"`
	ActionType           string  `json:"actionType"`
	Priority             int     `json:"priority"`
	Enabled              bool    `json:"enabled"`
	TenantID             string  `json:"tenantId"`
	LatestVersionID      string  `json:"latestVersionId,omitempty"`
	LatestVersionNum     int     `json:"latestVersionNum,omitempty"`
	LatestVersionSHA256  string  `json:"latestVersionSha256,omitempty"`
	LatestApprovalStatus string  `json:"latestApprovalStatus,omitempty"`
	ProdVersionID        string  `json:"prodVersionId,omitempty"`
	ProdVersionNum       int     `json:"prodVersionNum,omitempty"`
	CurrentEnv           string  `json:"currentEnv,omitempty"`
	CanaryPercent        float64 `json:"canaryPercent,omitempty"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
}

// WorkflowRuleVersion represents an immutable historical snapshot in workflow_rule_versions.
type WorkflowRuleVersion struct {
	VersionID           string `json:"versionId"`
	RuleID              string `json:"ruleId"`
	TenantID            string `json:"tenantId"`
	EnvironmentID       string `json:"environmentId"`
	VersionNumber       int    `json:"versionNumber"`
	RuleName            string `json:"ruleName"`
	RuleType            string `json:"ruleType"`
	ConditionExpression string `json:"conditionExpression"`
	ActionType          string `json:"actionType"`
	Priority            int    `json:"priority"`
	Enabled             bool   `json:"enabled"`
	RuleDefinitionJSON  string `json:"ruleDefinitionJson"`
	DefinitionSHA256    string `json:"definitionSha256"`
	CreatedBy           string `json:"createdBy"`
	CreatedAt           string `json:"createdAt"`
	IsActive            bool   `json:"isActive"`
	ChangeReason        string `json:"changeReason"`
	ApprovalStatus      string `json:"approvalStatus,omitempty"`
	ApprovedBy          string `json:"approvedBy,omitempty"`
	ApprovedAt          string `json:"approvedAt,omitempty"`
	ApprovedSHA256      string `json:"approvedSha256,omitempty"`
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
	RuleID           string   `json:"ruleId"`
	VersionID        string   `json:"versionId"`
	DefinitionSHA256 string   `json:"definitionSha256,omitempty"`
	Score            float64  `json:"score"`
	Level            string   `json:"level"` // LOW | MEDIUM | HIGH | CRITICAL
	BlocksProd       bool     `json:"blocksProd"`
	Factors          []string `json:"factors"`
}

// CanaryStatus reports FNV-1a deterministic canary rollout configuration for a rule.
type CanaryStatus struct {
	ID                   string  `json:"id"`
	RuleID               string  `json:"ruleId"`
	VersionID            string  `json:"versionId"`
	SampleSizePercentage float64 `json:"sampleSizePercentage"`
	RolloutStage         int     `json:"rolloutStage"`
	DeploymentStatus     string  `json:"deploymentStatus"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt,omitempty"`
	CompletedAt          string  `json:"completedAt,omitempty"`
}

// CanaryCohortMetrics captures execution, cost, and latency metrics for one cohort (canary or control).
type CanaryCohortMetrics struct {
	AttemptedCount int64   `json:"attemptedCount"`
	SuccessCount   int64   `json:"successCount"`
	FailureCount   int64   `json:"failureCount"`
	SkippedCount   int64   `json:"skippedCount"`
	CostUnits      float64 `json:"costUnits"`
	AvgDurationMs  float64 `json:"avgDurationMs"`
	FailureRate    float64 `json:"failureRate"`
	ExecutionCount int64   `json:"executionCount"`
}

// CanaryMonitoringSummary aggregates canary vs control telemetry and threshold health for a rule version.
type CanaryMonitoringSummary struct {
	RuleID              string              `json:"ruleId"`
	VersionID           string              `json:"versionId"`
	SampleSizePercentage float64            `json:"sampleSizePercentage"`
	RolloutStage        int                 `json:"rolloutStage"`
	DeploymentStatus    string              `json:"deploymentStatus"`
	WindowMinutes       int                 `json:"windowMinutes"`
	Canary              CanaryCohortMetrics `json:"canary"`
	Control             CanaryCohortMetrics `json:"control"`
	CostDeltaUnits      float64             `json:"costDeltaUnits"`
	CostIncreasePct     float64             `json:"costIncreasePct"`
	DurationDeltaMs     float64             `json:"durationDeltaMs"`
	DurationIncreasePct float64             `json:"durationIncreasePct"`
	HealthStatus        string              `json:"healthStatus"` // HEALTHY | DEGRADED | INSUFFICIENT_SAMPLES | ROLLED_BACK | COMPLETED
}

// CanaryRollbackThreshold mirrors canary_rollback_thresholds.
type CanaryRollbackThreshold struct {
	ID                        string  `json:"id"`
	MinSampleSize             int     `json:"minSampleSize"`
	MaxFailureRate            float64 `json:"maxFailureRate"`
	MaxCostIncreasePct        float64 `json:"maxCostIncreasePct"`
	MaxDurationIncreasePct    float64 `json:"maxDurationIncreasePct"`
	MonitoringWindowMinutes   int     `json:"monitoringWindowMinutes"`
	UpdatedAt                 string  `json:"updatedAt"`
}

// AutoRollbackResult reports the outcome of two-stage automatic canary evaluation & recovery.
type AutoRollbackResult struct {
	RuleID            string   `json:"ruleId"`
	FailedVersionID   string   `json:"failedVersionId"`
	Triggered         bool     `json:"triggered"`
	BreachReasons     []string `json:"breachReasons"`
	Stage1Disabled    bool     `json:"stage1Disabled"`
	Stage2Restored    bool     `json:"stage2Restored"`
	RestoredVersionID string   `json:"restoredVersionId,omitempty"`
	AlertID           string   `json:"alertId,omitempty"`
}

// OperationalAlert mirrors automation_stability_alert rows surfaced in the Automation Rules console.
type OperationalAlert struct {
	ID             string `json:"id"`
	AlertType      string `json:"alertType"`
	Severity       string `json:"severity"`
	RuleID         string `json:"ruleId"`
	VersionID      string `json:"versionId"`
	Message        string `json:"message"`
	DetailsJSON    string `json:"detailsJson"`
	CreatedAt      string `json:"createdAt"`
	Acknowledged   bool   `json:"acknowledged"`
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
	AcknowledgedAt string `json:"acknowledgedAt,omitempty"`
}

// AutomationCostLimit mirrors automation_cost_limits.
type AutomationCostLimit struct {
	ID                         string  `json:"id"`
	MaxCostUnitsPerCycle       float64 `json:"maxCostUnitsPerCycle"`
	MaxExecutionTimePerCycleMs int64   `json:"maxExecutionTimePerCycleMs"`
	MaxRecordsProcessedCycle   int64   `json:"maxRecordsProcessedPerCycle"`
	UpdatedAt                  string  `json:"updatedAt"`
}

// RuleDryRunMatch previews one exception case that would be affected by a rule version.
type RuleDryRunMatch struct {
	ExceptionCaseID string `json:"exceptionCaseId"`
	ExceptionType   string `json:"exceptionType"`
	EntityID        string `json:"entityId"`
	CurrentStatus   string `json:"currentStatus"`
	CurrentPriority string `json:"currentPriority"`
	ProposedAction  string `json:"proposedAction"`
	CanaryIncluded  bool   `json:"canaryIncluded"`
	AlreadyApplied  bool   `json:"alreadyApplied"`
}

// RuleDryRunResult reports a zero-mutation simulation of a rule version against live open exceptions.
type RuleDryRunResult struct {
	RuleID                 string             `json:"ruleId"`
	VersionID              string             `json:"versionId"`
	DefinitionSHA256       string             `json:"definitionSha256"`
	ActionType             string             `json:"actionType"`
	RuleEnabled            bool               `json:"ruleEnabled"`
	CanaryPercent          float64            `json:"canaryPercent"`
	TotalOpenCases         int                `json:"totalOpenCases"`
	EligibleCount          int                `json:"eligibleCount"`
	CanaryRoutedCount      int                `json:"canaryRoutedCount"`
	SkippedIdempotentCount int                `json:"skippedIdempotentCount"`
	EstimatedCostUnits     float64            `json:"estimatedCostUnits"`
	CostBudgetCap          float64            `json:"costBudgetCap"`
	RecordsBudgetCap       int64              `json:"recordsBudgetCap"`
	WithinBudget           bool               `json:"withinBudget"`
	Risk                   RuleRiskAssessment `json:"risk"`
	Samples                []RuleDryRunMatch  `json:"samples"`
}

// BackgroundJob represents one scheduled system job in workflow_background_jobs.
type BackgroundJob struct {
	JobID               string   `json:"jobId"`
	JobName             string   `json:"jobName"`
	JobType             string   `json:"jobType"`
	ScheduleType        string   `json:"scheduleType"`
	IsEnabled           bool     `json:"isEnabled"`
	ExpectedDurationMs  int64    `json:"expectedDurationMs"`
	MaxRetries          int      `json:"maxRetries"`
	RetryDelaySec       int      `json:"retryDelaySec"`
	LastStatus          string   `json:"lastStatus,omitempty"`
	LastStartedAt       string   `json:"lastStartedAt,omitempty"`
	LastCompletedAt     string   `json:"lastCompletedAt,omitempty"`
	LastErrorMessage    string   `json:"lastErrorMessage,omitempty"`
	GuardStopped        bool     `json:"guardStopped,omitempty"`
	RecoveryAttempts24h int      `json:"recoveryAttempts24h,omitempty"`
	DependsOn           []string `json:"dependsOn,omitempty"`
	NextExpectedAt      string   `json:"nextExpectedAt,omitempty"`
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

// RuleDeploymentHealth describes per-rule deployment state, telemetry metrics, and explained health status.
type RuleDeploymentHealth struct {
	RuleID                 string   `json:"ruleId"`
	RuleName               string   `json:"ruleName"`
	RuleType               string   `json:"ruleType"`
	ActionType             string   `json:"actionType"`
	Enabled                bool     `json:"enabled"`
	CurrentEnv             string   `json:"currentEnv"`
	ProdVersionID          string   `json:"prodVersionId,omitempty"`
	ProdVersionNum         int      `json:"prodVersionNum,omitempty"`
	LatestVersionID        string   `json:"latestVersionId,omitempty"`
	LatestVersionNum       int      `json:"latestVersionNum,omitempty"`
	LatestApprovalStatus   string   `json:"latestApprovalStatus,omitempty"`
	LatestDeployedAt       string   `json:"latestDeployedAt,omitempty"`
	CanaryStatus           string   `json:"canaryStatus,omitempty"`
	CanaryVersionID        string   `json:"canaryVersionId,omitempty"`
	CanaryPercent          float64  `json:"canaryPercent,omitempty"`
	CanaryRolloutStage     int      `json:"canaryRolloutStage,omitempty"`
	HealthStatus           string   `json:"healthStatus"` // HEALTHY | WARNING | CRITICAL | DISABLED | UNKNOWN
	HealthReasons          []string `json:"healthReasons"`
	AttemptedCount         int64    `json:"attemptedCount"`
	SuccessCount           int64    `json:"successCount"`
	FailureCount           int64    `json:"failureCount"`
	SkippedCount           int64    `json:"skippedCount"`
	FailureRate            float64  `json:"failureRate"`
	CostUnits              float64  `json:"costUnits"`
	AvgDurationMs          float64  `json:"avgDurationMs"`
	CostIncreasePct        float64  `json:"costIncreasePct"`
	DurationIncreasePct    float64  `json:"durationIncreasePct"`
	OpenAlertCount         int      `json:"openAlertCount"`
	AcknowledgedAlertCount int      `json:"acknowledgedAlertCount"`
	HasRecentRollback      bool     `json:"hasRecentRollback"`
}

// RollbackEventRecord mirrors persisted rollback and recovery events from rollback_action_log.
type RollbackEventRecord struct {
	ID           string `json:"id"`
	RuleID       string `json:"ruleId"`
	RollbackType string `json:"rollbackType"`
	TargetEntity string `json:"targetEntity"`
	PerformedBy  string `json:"performedBy"`
	PerformedAt  string `json:"performedAt"`
	DetailsJSON  string `json:"detailsJson"`
}

// DeploymentHealthOverview provides a read-only consolidated snapshot of rule deployments, health, alerts, and rollbacks.
type DeploymentHealthOverview struct {
	TotalRules                    int                    `json:"totalRules"`
	ActiveProductionRules         int                    `json:"activeProductionRules"`
	RulesAwaitingApproval         int                    `json:"rulesAwaitingApproval"`
	ActiveCanaryDeployments       int                    `json:"activeCanaryDeployments"`
	CompletedCanaryDeployments    int                    `json:"completedCanaryDeployments"`
	FailedOrRolledBackDeployments int                    `json:"failedOrRolledBackDeployments"`
	CurrentProductionRuleID       string                 `json:"currentProductionRuleId,omitempty"`
	CurrentProductionVersionID    string                 `json:"currentProductionVersionId,omitempty"`
	CurrentProductionVersionNum   int                    `json:"currentProductionVersionNum,omitempty"`
	LatestDeploymentTimestamp     string                 `json:"latestDeploymentTimestamp,omitempty"`
	OverallHealth                 string                 `json:"overallHealth"` // HEALTHY | WARNING | CRITICAL | DISABLED | UNKNOWN
	HealthSummaryReason           string                 `json:"healthSummaryReason"`
	WindowMinutes                 int                    `json:"windowMinutes"`
	Thresholds                    CanaryRollbackThreshold `json:"thresholds"`
	RuleDeployments               []RuleDeploymentHealth `json:"ruleDeployments"`
	RecentAlerts                  []OperationalAlert     `json:"recentAlerts"`
	RecentRollbacks               []RollbackEventRecord  `json:"recentRollbacks"`
}

// RuleVersionSummary describes one immutable version's attributes, approval, deployment state, and risk profile.
type RuleVersionSummary struct {
	VersionID           string   `json:"versionId"`
	VersionNumber       int      `json:"versionNumber"`
	RuleName            string   `json:"ruleName"`
	RuleType            string   `json:"ruleType"`
	ActionType          string   `json:"actionType"`
	ConditionExpression string   `json:"conditionExpression"`
	Priority            int      `json:"priority"`
	Enabled             bool     `json:"enabled"`
	DefinitionSHA256    string   `json:"definitionSha256"`
	RiskLevel           string   `json:"riskLevel"`
	RiskScore           float64  `json:"riskScore"`
	BlocksProd          bool     `json:"blocksProd"`
	RiskFactors         []string `json:"riskFactors"`
	ApprovalStatus      string   `json:"approvalStatus"`
	DeploymentStatus    string   `json:"deploymentStatus"`
	CreatedAt           string   `json:"createdAt"`
	CreatedBy           string   `json:"createdBy"`
}

// RuleVersionFieldDiff represents a single attribute difference between base and target rule versions.
type RuleVersionFieldDiff struct {
	Field       string `json:"field"`
	BaseValue   string `json:"baseValue"`
	TargetValue string `json:"targetValue"`
	Changed     bool   `json:"changed"`
	ImpactNote  string `json:"impactNote,omitempty"`
}

// DryRunComparisonDelta compares simulated execution impact of base vs target versions under equivalent assumptions.
type DryRunComparisonDelta struct {
	AssumedCanaryPercent  float64 `json:"assumedCanaryPercent"`
	EvaluatedOpenCases    int     `json:"evaluatedOpenCases"`
	BaseMatchingRecords   int     `json:"baseMatchingRecords"`
	TargetMatchingRecords int     `json:"targetMatchingRecords"`
	MatchingRecordsDelta  int     `json:"matchingRecordsDelta"`
	BaseAffectedRecords   int     `json:"baseAffectedRecords"`
	TargetAffectedRecords int     `json:"targetAffectedRecords"`
	AffectedRecordsDelta  int     `json:"affectedRecordsDelta"`
	BaseEstimatedCost     float64 `json:"baseEstimatedCost"`
	TargetEstimatedCost   float64 `json:"targetEstimatedCost"`
	EstimatedCostDelta    float64 `json:"estimatedCostDelta"`
	BaseSkippedRecords    int     `json:"baseSkippedRecords"`
	TargetSkippedRecords  int     `json:"targetSkippedRecords"`
	SkippedRecordsDelta   int     `json:"skippedRecordsDelta"`
	BaseWithinBudget      bool    `json:"baseWithinBudget"`
	TargetWithinBudget    bool    `json:"targetWithinBudget"`
}

// RuleVersionComparison compares two immutable versions of the same rule, their risk, and read-only dry-run impact.
type RuleVersionComparison struct {
	RuleID                string                 `json:"ruleId"`
	Base                  RuleVersionSummary     `json:"base"`
	Target                RuleVersionSummary     `json:"target"`
	ChangedFields         []string               `json:"changedFields"`
	FieldDiffs            []RuleVersionFieldDiff `json:"fieldDiffs"`
	ImportantChanges      []string               `json:"importantChanges"`
	RiskScoreDelta        float64                `json:"riskScoreDelta"`
	RiskLevelChanged      bool                   `json:"riskLevelChanged"`
	RiskDifferenceSummary string                 `json:"riskDifferenceSummary"`
	DryRunImpact          DryRunComparisonDelta  `json:"dryRunImpact"`
	ComparedAt            string                 `json:"comparedAt"`
}
