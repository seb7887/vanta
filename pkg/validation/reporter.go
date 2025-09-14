package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"sync"
	"time"

	"vanta/pkg/openapi"
)

type Reporter struct {
	mu               sync.RWMutex
	coverage         *CoverageReport
	compliance       *ComplianceReport
	endpointStats    map[string]*EndpointStats
	requestHistory   []RequestRecord
	responseHistory  []ResponseRecord
	config           *ReporterConfig
}

type ReporterConfig struct {
	MaxHistorySize   int           `json:"max_history_size" yaml:"max_history_size"`
	ReportInterval   time.Duration `json:"report_interval" yaml:"report_interval"`
	AutoSave         bool          `json:"auto_save" yaml:"auto_save"`
	SavePath         string        `json:"save_path" yaml:"save_path"`
	IncludeExamples  bool          `json:"include_examples" yaml:"include_examples"`
}

type CoverageReport struct {
	TotalEndpoints    int                        `json:"total_endpoints"`
	CoveredEndpoints  int                        `json:"covered_endpoints"`
	CoveragePercent   float64                    `json:"coverage_percent"`
	Endpoints         map[string]*EndpointStats  `json:"endpoints"`
	GeneratedAt       time.Time                  `json:"generated_at"`
	Summary           CoverageSummary            `json:"summary"`
}

type CoverageSummary struct {
	ByMethod     map[string]int `json:"by_method"`
	ByStatusCode map[string]int `json:"by_status_code"`
	ByPath       map[string]int `json:"by_path"`
}

type ComplianceReport struct {
	TotalRequests      int                   `json:"total_requests"`
	ValidRequests      int                   `json:"valid_requests"`
	InvalidRequests    int                   `json:"invalid_requests"`
	TotalResponses     int                   `json:"total_responses"`
	ValidResponses     int                   `json:"valid_responses"`
	InvalidResponses   int                   `json:"invalid_responses"`
	CompliancePercent  float64              `json:"compliance_percent"`
	GeneratedAt        time.Time            `json:"generated_at"`
	ErrorsByType       map[string]int       `json:"errors_by_type"`
	ErrorsByEndpoint   map[string]int       `json:"errors_by_endpoint"`
	Violations         []ComplianceViolation `json:"violations"`
}

type ComplianceViolation struct {
	Endpoint      string    `json:"endpoint"`
	Method        string    `json:"method"`
	Type          string    `json:"type"`
	Message       string    `json:"message"`
	Count         int       `json:"count"`
	LastSeen      time.Time `json:"last_seen"`
	Examples      []string  `json:"examples,omitempty"`
}

type EndpointStats struct {
	Path             string            `json:"path"`
	Method           string            `json:"method"`
	OperationID      string            `json:"operation_id,omitempty"`
	RequestCount     int               `json:"request_count"`
	ResponseCount    int               `json:"response_count"`
	ValidRequests    int               `json:"valid_requests"`
	InvalidRequests  int               `json:"invalid_requests"`
	ValidResponses   int               `json:"valid_responses"`
	InvalidResponses int               `json:"invalid_responses"`
	StatusCodes      map[string]int    `json:"status_codes"`
	ErrorTypes       map[string]int    `json:"error_types"`
	FirstSeen        time.Time         `json:"first_seen"`
	LastSeen         time.Time         `json:"last_seen"`
	AverageLatency   time.Duration     `json:"average_latency"`
	Examples         []RequestExample  `json:"examples,omitempty"`
}

type RequestExample struct {
	RequestID   string      `json:"request_id"`
	Timestamp   time.Time   `json:"timestamp"`
	Valid       bool        `json:"valid"`
	StatusCode  int         `json:"status_code,omitempty"`
	ErrorTypes  []string    `json:"error_types,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        interface{} `json:"body,omitempty"`
}

type RequestRecord struct {
	RequestID  string                `json:"request_id"`
	Endpoint   *EndpointInfo         `json:"endpoint"`
	Timestamp  time.Time             `json:"timestamp"`
	Result     *ValidationResult     `json:"result"`
}

type ResponseRecord struct {
	RequestID  string                     `json:"request_id"`
	Endpoint   *EndpointInfo              `json:"endpoint"`
	Timestamp  time.Time                  `json:"timestamp"`
	Result     *ResponseValidationResult  `json:"result"`
}

func NewReporter() *Reporter {
	config := &ReporterConfig{
		MaxHistorySize:  1000,
		ReportInterval:  5 * time.Minute,
		AutoSave:        true,
		SavePath:        "./reports",
		IncludeExamples: true,
	}

	return &Reporter{
		coverage:        &CoverageReport{Endpoints: make(map[string]*EndpointStats)},
		compliance:      &ComplianceReport{ErrorsByType: make(map[string]int), ErrorsByEndpoint: make(map[string]int)},
		endpointStats:   make(map[string]*EndpointStats),
		requestHistory:  make([]RequestRecord, 0, config.MaxHistorySize),
		responseHistory: make([]ResponseRecord, 0, config.MaxHistorySize),
		config:          config,
	}
}

func (r *Reporter) RecordRequest(ctx context.Context, endpoint *openapi.Endpoint, result *ValidationResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	endpointKey := fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)

	// Initialize endpoint stats if not exists
	if _, exists := r.endpointStats[endpointKey]; !exists {
		r.endpointStats[endpointKey] = &EndpointStats{
			Path:         endpoint.Path,
			Method:       endpoint.Method,
			OperationID:  endpoint.OperationID,
			StatusCodes:  make(map[string]int),
			ErrorTypes:   make(map[string]int),
			FirstSeen:    now,
			Examples:     make([]RequestExample, 0),
		}
	}

	stats := r.endpointStats[endpointKey]
	stats.RequestCount++
	stats.LastSeen = now

	if result.Valid {
		stats.ValidRequests++
		r.compliance.ValidRequests++
	} else {
		stats.InvalidRequests++
		r.compliance.InvalidRequests++

		// Record error types
		for _, err := range result.Errors {
			stats.ErrorTypes[err.Type]++
			r.compliance.ErrorsByType[err.Type]++
			r.compliance.ErrorsByEndpoint[endpointKey]++
		}
	}

	r.compliance.TotalRequests++

	// Add example if configured
	if r.config.IncludeExamples && len(stats.Examples) < 10 {
		example := RequestExample{
			RequestID: result.RequestID,
			Timestamp: result.Timestamp,
			Valid:     result.Valid,
		}

		if !result.Valid {
			for _, err := range result.Errors {
				example.ErrorTypes = append(example.ErrorTypes, err.Type)
			}
		}

		stats.Examples = append(stats.Examples, example)
	}

	// Add to request history
	record := RequestRecord{
		RequestID: result.RequestID,
		Endpoint:  result.Endpoint,
		Timestamp: result.Timestamp,
		Result:    result,
	}

	r.addRequestToHistory(record)
	r.updateComplianceViolations(result)
}

func (r *Reporter) RecordResponse(ctx context.Context, endpoint *openapi.Endpoint, result *ResponseValidationResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	endpointKey := fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)

	// Initialize endpoint stats if not exists
	if _, exists := r.endpointStats[endpointKey]; !exists {
		r.endpointStats[endpointKey] = &EndpointStats{
			Path:        endpoint.Path,
			Method:      endpoint.Method,
			OperationID: endpoint.OperationID,
			StatusCodes: make(map[string]int),
			ErrorTypes:  make(map[string]int),
			FirstSeen:   now,
			Examples:    make([]RequestExample, 0),
		}
	}

	stats := r.endpointStats[endpointKey]
	stats.ResponseCount++
	stats.LastSeen = now

	// Record status code
	statusCodeStr := fmt.Sprintf("%d", result.StatusCode)
	stats.StatusCodes[statusCodeStr]++

	if result.Valid {
		stats.ValidResponses++
		r.compliance.ValidResponses++
	} else {
		stats.InvalidResponses++
		r.compliance.InvalidResponses++

		// Record error types
		for _, err := range result.Errors {
			stats.ErrorTypes[err.Type]++
			r.compliance.ErrorsByType[err.Type]++
			r.compliance.ErrorsByEndpoint[endpointKey]++
		}
	}

	r.compliance.TotalResponses++

	// Add to response history
	record := ResponseRecord{
		RequestID: result.RequestID,
		Endpoint:  result.Endpoint,
		Timestamp: result.Timestamp,
		Result:    result,
	}

	r.addResponseToHistory(record)
	r.updateResponseComplianceViolations(result)
}

func (r *Reporter) GenerateCoverageReport(spec *openapi.Specification) *CoverageReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report := &CoverageReport{
		Endpoints:   make(map[string]*EndpointStats),
		GeneratedAt: time.Now(),
		Summary: CoverageSummary{
			ByMethod:     make(map[string]int),
			ByStatusCode: make(map[string]int),
			ByPath:       make(map[string]int),
		},
	}

    // Count total endpoints from specification
    totalEndpoints := 0
    for _, pathItem := range spec.Paths {
		if pathItem.GET != nil {
			totalEndpoints++
		}
		if pathItem.POST != nil {
			totalEndpoints++
		}
		if pathItem.PUT != nil {
			totalEndpoints++
		}
		if pathItem.DELETE != nil {
			totalEndpoints++
		}
		if pathItem.PATCH != nil {
			totalEndpoints++
		}
	}

	report.TotalEndpoints = totalEndpoints

	// Copy endpoint stats and calculate coverage
	coveredEndpoints := 0
	for key, stats := range r.endpointStats {
		if stats.RequestCount > 0 {
			coveredEndpoints++
		}
		report.Endpoints[key] = stats

		// Update summary
		report.Summary.ByMethod[stats.Method]++
		for statusCode := range stats.StatusCodes {
			report.Summary.ByStatusCode[statusCode]++
		}
		report.Summary.ByPath[stats.Path]++
	}

	report.CoveredEndpoints = coveredEndpoints
	if totalEndpoints > 0 {
		report.CoveragePercent = float64(coveredEndpoints) / float64(totalEndpoints) * 100
	}

	return report
}

func (r *Reporter) GenerateComplianceReport() *ComplianceReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report := &ComplianceReport{
		TotalRequests:     r.compliance.TotalRequests,
		ValidRequests:     r.compliance.ValidRequests,
		InvalidRequests:   r.compliance.InvalidRequests,
		TotalResponses:    r.compliance.TotalResponses,
		ValidResponses:    r.compliance.ValidResponses,
		InvalidResponses:  r.compliance.InvalidResponses,
		GeneratedAt:       time.Now(),
		ErrorsByType:      make(map[string]int),
		ErrorsByEndpoint:  make(map[string]int),
		Violations:        make([]ComplianceViolation, 0, len(r.compliance.Violations)),
	}

	// Copy error statistics
	for errType, count := range r.compliance.ErrorsByType {
		report.ErrorsByType[errType] = count
	}

	for endpoint, count := range r.compliance.ErrorsByEndpoint {
		report.ErrorsByEndpoint[endpoint] = count
	}

	// Copy violations
	report.Violations = append(report.Violations, r.compliance.Violations...)

	// Calculate compliance percentage
	totalValidations := report.TotalRequests + report.TotalResponses
	totalValid := report.ValidRequests + report.ValidResponses
	if totalValidations > 0 {
		report.CompliancePercent = float64(totalValid) / float64(totalValidations) * 100
	}

	return report
}

func (r *Reporter) SaveReports(spec *openapi.Specification, format string) error {
	if err := os.MkdirAll(r.config.SavePath, 0755); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Generate reports
	coverageReport := r.GenerateCoverageReport(spec)
	complianceReport := r.GenerateComplianceReport()

	timestamp := time.Now().Format("2006-01-02_15-04-05")

	switch format {
	case "json":
		if err := r.saveJSONReports(coverageReport, complianceReport, timestamp); err != nil {
			return err
		}
	case "html":
		if err := r.saveHTMLReports(coverageReport, complianceReport, timestamp); err != nil {
			return err
		}
	case "junit":
		if err := r.saveJUnitReport(complianceReport, timestamp); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported report format: %s", format)
	}

	return nil
}

func (r *Reporter) saveJSONReports(coverage *CoverageReport, compliance *ComplianceReport, timestamp string) error {
	// Save coverage report
	coverageFile := fmt.Sprintf("%s/coverage_%s.json", r.config.SavePath, timestamp)
	coverageData, err := json.MarshalIndent(coverage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal coverage report: %w", err)
	}

	if err := os.WriteFile(coverageFile, coverageData, 0644); err != nil {
		return fmt.Errorf("failed to save coverage report: %w", err)
	}

	// Save compliance report
	complianceFile := fmt.Sprintf("%s/compliance_%s.json", r.config.SavePath, timestamp)
	complianceData, err := json.MarshalIndent(compliance, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal compliance report: %w", err)
	}

	if err := os.WriteFile(complianceFile, complianceData, 0644); err != nil {
		return fmt.Errorf("failed to save compliance report: %w", err)
	}

	return nil
}

func (r *Reporter) saveHTMLReports(coverage *CoverageReport, compliance *ComplianceReport, timestamp string) error {
	// HTML templates for reports
	coverageTemplate := `
<!DOCTYPE html>
<html>
<head>
    <title>API Coverage Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f4f4f4; padding: 20px; border-radius: 5px; }
        .summary { display: flex; gap: 20px; margin: 20px 0; }
        .card { background: #fff; border: 1px solid #ddd; padding: 15px; border-radius: 5px; flex: 1; }
        .percentage { font-size: 2em; font-weight: bold; color: #2ecc71; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f4f4f4; }
        .covered { color: #2ecc71; }
        .not-covered { color: #e74c3c; }
    </style>
</head>
<body>
    <div class="header">
        <h1>API Coverage Report</h1>
        <p>Generated at: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
    </div>

    <div class="summary">
        <div class="card">
            <h3>Overall Coverage</h3>
            <div class="percentage">{{printf "%.1f" .CoveragePercent}}%</div>
            <p>{{.CoveredEndpoints}} of {{.TotalEndpoints}} endpoints covered</p>
        </div>
    </div>

    <h2>Endpoint Details</h2>
    <table>
        <tr>
            <th>Method</th>
            <th>Path</th>
            <th>Requests</th>
            <th>Valid</th>
            <th>Invalid</th>
            <th>Status</th>
        </tr>
        {{range $key, $endpoint := .Endpoints}}
        <tr>
            <td>{{$endpoint.Method}}</td>
            <td>{{$endpoint.Path}}</td>
            <td>{{$endpoint.RequestCount}}</td>
            <td class="covered">{{$endpoint.ValidRequests}}</td>
            <td class="not-covered">{{$endpoint.InvalidRequests}}</td>
            <td>{{if gt $endpoint.RequestCount 0}}<span class="covered">Covered</span>{{else}}<span class="not-covered">Not Covered</span>{{end}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>`

	tmpl, err := template.New("coverage").Parse(coverageTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse coverage template: %w", err)
	}

	coverageFile := fmt.Sprintf("%s/coverage_%s.html", r.config.SavePath, timestamp)
	file, err := os.Create(coverageFile)
	if err != nil {
		return fmt.Errorf("failed to create coverage HTML file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, coverage); err != nil {
		return fmt.Errorf("failed to execute coverage template: %w", err)
	}

	return nil
}

func (r *Reporter) saveJUnitReport(compliance *ComplianceReport, timestamp string) error {
	junitTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="API Compliance" tests="{{.TotalRequests}}" failures="{{.InvalidRequests}}" errors="0" time="0">
    {{range .Violations}}
    <testcase name="{{.Endpoint}} {{.Method}}" classname="{{.Type}}">
        {{if gt .Count 0}}
        <failure message="{{.Message}}" type="{{.Type}}">
            {{.Message}} (occurred {{.Count}} times)
        </failure>
        {{end}}
    </testcase>
    {{end}}
</testsuite>`

	tmpl, err := template.New("junit").Parse(junitTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse JUnit template: %w", err)
	}

	junitFile := fmt.Sprintf("%s/compliance_%s.xml", r.config.SavePath, timestamp)
	file, err := os.Create(junitFile)
	if err != nil {
		return fmt.Errorf("failed to create JUnit XML file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, compliance); err != nil {
		return fmt.Errorf("failed to execute JUnit template: %w", err)
	}

	return nil
}

func (r *Reporter) addRequestToHistory(record RequestRecord) {
	if len(r.requestHistory) >= r.config.MaxHistorySize {
		// Remove oldest record
		r.requestHistory = r.requestHistory[1:]
	}
	r.requestHistory = append(r.requestHistory, record)
}

func (r *Reporter) addResponseToHistory(record ResponseRecord) {
	if len(r.responseHistory) >= r.config.MaxHistorySize {
		// Remove oldest record
		r.responseHistory = r.responseHistory[1:]
	}
	r.responseHistory = append(r.responseHistory, record)
}

func (r *Reporter) updateComplianceViolations(result *ValidationResult) {
	if result.Valid || result.Endpoint == nil {
		return
	}

	endpointKey := fmt.Sprintf("%s %s", result.Endpoint.Method, result.Endpoint.Path)

	// Group errors by type and update violation counts
	for _, err := range result.Errors {
		found := false
		for i := range r.compliance.Violations {
			violation := &r.compliance.Violations[i]
			if violation.Endpoint == endpointKey && violation.Type == err.Type {
				violation.Count++
				violation.LastSeen = time.Now()
				if len(violation.Examples) < 3 && result.RequestID != "" {
					violation.Examples = append(violation.Examples, result.RequestID)
				}
				found = true
				break
			}
		}

		if !found {
			violation := ComplianceViolation{
				Endpoint:  endpointKey,
				Method:    result.Endpoint.Method,
				Type:      err.Type,
				Message:   err.Message,
				Count:     1,
				LastSeen:  time.Now(),
				Examples:  []string{},
			}

			if result.RequestID != "" {
				violation.Examples = append(violation.Examples, result.RequestID)
			}

			r.compliance.Violations = append(r.compliance.Violations, violation)
		}
	}
}

func (r *Reporter) updateResponseComplianceViolations(result *ResponseValidationResult) {
	if result.Valid || result.Endpoint == nil {
		return
	}

	endpointKey := fmt.Sprintf("%s %s", result.Endpoint.Method, result.Endpoint.Path)

	// Group errors by type and update violation counts
	for _, err := range result.Errors {
		found := false
		for i := range r.compliance.Violations {
			violation := &r.compliance.Violations[i]
			if violation.Endpoint == endpointKey && violation.Type == err.Type {
				violation.Count++
				violation.LastSeen = time.Now()
				if len(violation.Examples) < 3 && result.RequestID != "" {
					violation.Examples = append(violation.Examples, result.RequestID)
				}
				found = true
				break
			}
		}

		if !found {
			violation := ComplianceViolation{
				Endpoint:  endpointKey,
				Method:    result.Endpoint.Method,
				Type:      err.Type,
				Message:   err.Message,
				Count:     1,
				LastSeen:  time.Now(),
				Examples:  []string{},
			}

			if result.RequestID != "" {
				violation.Examples = append(violation.Examples, result.RequestID)
			}

			r.compliance.Violations = append(r.compliance.Violations, violation)
		}
	}
}

func (r *Reporter) GetTopViolations(limit int) []ComplianceViolation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	violations := make([]ComplianceViolation, len(r.compliance.Violations))
	copy(violations, r.compliance.Violations)

	// Sort by count (descending)
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Count > violations[j].Count
	})

	if limit > 0 && len(violations) > limit {
		violations = violations[:limit]
	}

	return violations
}

func (r *Reporter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.coverage = &CoverageReport{Endpoints: make(map[string]*EndpointStats)}
	r.compliance = &ComplianceReport{ErrorsByType: make(map[string]int), ErrorsByEndpoint: make(map[string]int)}
	r.endpointStats = make(map[string]*EndpointStats)
	r.requestHistory = make([]RequestRecord, 0, r.config.MaxHistorySize)
	r.responseHistory = make([]ResponseRecord, 0, r.config.MaxHistorySize)
}
