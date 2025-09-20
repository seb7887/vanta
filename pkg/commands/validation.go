package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"github.com/seb7887/vanta/pkg/openapi"
	"github.com/seb7887/vanta/pkg/validation"
)

func NewValidationCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validation and compliance commands",
		Long:  "Validate OpenAPI specifications and generate compliance reports",
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is mocker.yaml)")

	// Add subcommands
	cmd.AddCommand(NewValidateSpecCommand(ctx, logger, &configFile))
	cmd.AddCommand(NewValidateCoverageCommand(ctx, logger, &configFile))
	cmd.AddCommand(NewValidateComplianceCommand(ctx, logger, &configFile))

	return cmd
}

func NewValidateSpecCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		output    string
		format    string
		strict    bool
		examples  bool
	)

	cmd := &cobra.Command{
		Use:   "spec <openapi-file>",
		Short: "Validate OpenAPI specification",
		Long:  "Validate an OpenAPI specification file for correctness and completeness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specFile := args[0]

			// Load and parse OpenAPI specification
			spec, err := loadOpenAPISpec(specFile, logger)
			if err != nil {
				return fmt.Errorf("failed to load OpenAPI spec: %w", err)
			}

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create validation config
			validationCfg := cfg.Validation.ToValidationConfig()
			validationCfg.StrictMode = strict

			// Create validation manager (not used directly here, but ensures setup)
			_ = validation.NewValidationManager(spec, validationCfg)

			// Perform validation
			result := validateSpecification(spec, validationCfg, examples)

			// Format output
			var outputData []byte
			switch format {
			case "json":
				outputData, err = json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			case "text":
				outputData = formatValidationResultText(result)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}

			// Output result
			if output != "" {
				// Ensure directory exists
				if dir := filepath.Dir(output); dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}
				}

				if err := os.WriteFile(output, outputData, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Printf("Validation result written to: %s\n", output)
			} else {
				fmt.Print(string(outputData))
			}

			// Exit with error code if validation failed
			if !result.Valid {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format (json, text)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Enable strict validation mode")
	cmd.Flags().BoolVar(&examples, "examples", true, "Validate examples in the specification")

	return cmd
}

func NewValidateCoverageCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		output    string
		format    string
		threshold float64
		dataFile  string
	)

	cmd := &cobra.Command{
		Use:   "coverage <openapi-file>",
		Short: "Generate API coverage report",
		Long:  "Generate a coverage report showing which endpoints have been tested",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specFile := args[0]

			// Load and parse OpenAPI specification
			spec, err := loadOpenAPISpec(specFile, logger)
			if err != nil {
				return fmt.Errorf("failed to load OpenAPI spec: %w", err)
			}

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create reporter with persistent data
			var reporter *validation.Reporter
			if dataFile != "" {
				// Use custom data file
				reporterConfig := &validation.ReporterConfig{
					MaxHistorySize:   1000,
					ReportInterval:   5 * time.Minute,
					AutoSave:         true,
					SavePath:         "./reports",
					IncludeExamples:  true,
					PersistData:      true,
					DataFile:         dataFile,
					AutoSaveInterval: 30 * time.Second,
				}
				reporter = validation.NewReporterWithConfig(reporterConfig)
			} else {
				// Use configuration from config file
				reporterConfig := cfg.Validation.ToReporterConfig()
				reporter = validation.NewReporterWithConfig(reporterConfig)
			}

			// Generate coverage report
			coverageReport := reporter.GenerateCoverageReport(spec)

			// Format output based on requested format
			var outputData []byte

			switch format {
			case "json":
				outputData, err = json.MarshalIndent(coverageReport, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			case "html":
				outputData, err = formatCoverageReportHTML(coverageReport)
				if err != nil {
					return fmt.Errorf("failed to generate HTML report: %w", err)
				}
			case "text":
				outputData = formatCoverageReportText(coverageReport)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}

			// Output result
			if output != "" {
				// Ensure directory exists
				if dir := filepath.Dir(output); dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}
				}

				if err := os.WriteFile(output, outputData, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Printf("Coverage report written to: %s\n", output)
			} else {
				fmt.Print(string(outputData))
			}

			// Check coverage threshold
			if coverageReport.CoveragePercent < threshold {
				fmt.Printf("Coverage %0.1f%% is below threshold %0.1f%%\n",
					coverageReport.CoveragePercent, threshold)
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format (json, html, text)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.0, "Minimum coverage threshold (0-100)")
	cmd.Flags().StringVarP(&dataFile, "data-file", "d", "", "Validation data file (default: ./validation-data.json)")

	return cmd
}

func NewValidateComplianceCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		output    string
		format    string
		threshold float64
		dataFile  string
	)

	cmd := &cobra.Command{
		Use:   "compliance <openapi-file>",
		Short: "Generate compliance report",
		Long:  "Generate a compliance report showing validation errors and warnings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create reporter with persistent data
			var reporter *validation.Reporter
			if dataFile != "" {
				// Use custom data file
				reporterConfig := &validation.ReporterConfig{
					MaxHistorySize:   1000,
					ReportInterval:   5 * time.Minute,
					AutoSave:         true,
					SavePath:         "./reports",
					IncludeExamples:  true,
					PersistData:      true,
					DataFile:         dataFile,
					AutoSaveInterval: 30 * time.Second,
				}
				reporter = validation.NewReporterWithConfig(reporterConfig)
			} else {
				// Use configuration from config file
				reporterConfig := cfg.Validation.ToReporterConfig()
				reporter = validation.NewReporterWithConfig(reporterConfig)
			}

			// Generate compliance report
			complianceReport := reporter.GenerateComplianceReport()

			// Format output based on requested format
			var outputData []byte

			switch format {
			case "json":
				outputData, err = json.MarshalIndent(complianceReport, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			case "junit":
				outputData, err = formatComplianceReportJUnit(complianceReport)
				if err != nil {
					return fmt.Errorf("failed to generate JUnit report: %w", err)
				}
			case "text":
				outputData = formatComplianceReportText(complianceReport)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}

			// Output result
			if output != "" {
				// Ensure directory exists
				if dir := filepath.Dir(output); dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}
				}

				if err := os.WriteFile(output, outputData, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Printf("Compliance report written to: %s\n", output)
			} else {
				fmt.Print(string(outputData))
			}

			// Check compliance threshold
			if complianceReport.CompliancePercent < threshold {
				fmt.Printf("Compliance %0.1f%% is below threshold %0.1f%%\n",
					complianceReport.CompliancePercent, threshold)
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format (json, junit, text)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.0, "Minimum compliance threshold (0-100)")
	cmd.Flags().StringVarP(&dataFile, "data-file", "d", "", "Validation data file (default: ./validation-data.json)")

	return cmd
}

// Helper functions

type SpecValidationResult struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Info       map[string]interface{} `json:"info"`
}

func validateSpecification(spec *openapi.Specification, cfg *validation.Config, validateExamples bool) *SpecValidationResult {
	result := &SpecValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
		Info: map[string]interface{}{
			"version":      spec.Version,
			"title":        spec.Info.Title,
			"endpoints":    len(spec.Paths),
			"schemas":      len(spec.Schemas),
		},
	}

	// Basic validation
	if spec.Info.Title == "" {
		result.Errors = append(result.Errors, "API title is required")
		result.Valid = false
	}

	if spec.Info.Version == "" {
		result.Errors = append(result.Errors, "API version is required")
		result.Valid = false
	}

	if len(spec.Paths) == 0 {
		result.Warnings = append(result.Warnings, "No paths defined in specification")
	}

	// Additional validation logic would go here

	return result
}

func formatValidationResultText(result *SpecValidationResult) []byte {
	var output strings.Builder

	output.WriteString("OpenAPI Specification Validation\n")
	output.WriteString("================================\n\n")

	output.WriteString(fmt.Sprintf("Status: %s\n", map[bool]string{true: "VALID", false: "INVALID"}[result.Valid]))
	output.WriteString(fmt.Sprintf("Title: %s\n", result.Info["title"]))
	output.WriteString(fmt.Sprintf("Version: %s\n", result.Info["version"]))
	output.WriteString(fmt.Sprintf("Endpoints: %d\n", result.Info["endpoints"]))
	output.WriteString(fmt.Sprintf("Schemas: %d\n\n", result.Info["schemas"]))

	if len(result.Errors) > 0 {
		output.WriteString("Errors:\n")
		for _, err := range result.Errors {
			output.WriteString(fmt.Sprintf("  - %s\n", err))
		}
		output.WriteString("\n")
	}

	if len(result.Warnings) > 0 {
		output.WriteString("Warnings:\n")
		for _, warning := range result.Warnings {
			output.WriteString(fmt.Sprintf("  - %s\n", warning))
		}
		output.WriteString("\n")
	}

	return []byte(output.String())
}

func formatCoverageReportText(report *validation.CoverageReport) []byte {
	var output strings.Builder

	output.WriteString("API Coverage Report\n")
	output.WriteString("===================\n\n")

	output.WriteString(fmt.Sprintf("Overall Coverage: %.1f%%\n", report.CoveragePercent))
	output.WriteString(fmt.Sprintf("Covered Endpoints: %d/%d\n\n", report.CoveredEndpoints, report.TotalEndpoints))

	output.WriteString("Endpoint Details:\n")
	for key, endpoint := range report.Endpoints {
		status := "NOT COVERED"
		if endpoint.RequestCount > 0 {
			status = "COVERED"
		}
		output.WriteString(fmt.Sprintf("  %s - %s (%d requests)\n", key, status, endpoint.RequestCount))
	}

	return []byte(output.String())
}

func formatCoverageReportHTML(report *validation.CoverageReport) ([]byte, error) {
	// Generate HTML report using templates
	html := fmt.Sprintf(`
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
        table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f4f4f4; }
        .covered { color: #2ecc71; }
        .not-covered { color: #e74c3c; }
    </style>
</head>
<body>
    <div class="header">
        <h1>API Coverage Report</h1>
        <p>Generated at: %s</p>
    </div>

    <div class="summary">
        <div class="card">
            <h3>Overall Coverage</h3>
            <div class="percentage">%.1f%%</div>
            <p>%d of %d endpoints covered</p>
        </div>
    </div>
</body>
</html>`,
		report.GeneratedAt.Format("2006-01-02 15:04:05"),
		report.CoveragePercent,
		report.CoveredEndpoints,
		report.TotalEndpoints)

	return []byte(html), nil
}

func formatComplianceReportText(report *validation.ComplianceReport) []byte {
	var output strings.Builder

	output.WriteString("API Compliance Report\n")
	output.WriteString("=====================\n\n")

	output.WriteString(fmt.Sprintf("Compliance: %.1f%%\n", report.CompliancePercent))
	output.WriteString(fmt.Sprintf("Total Requests: %d\n", report.TotalRequests))
	output.WriteString(fmt.Sprintf("Valid Requests: %d\n", report.ValidRequests))
	output.WriteString(fmt.Sprintf("Invalid Requests: %d\n\n", report.InvalidRequests))

	if len(report.Violations) > 0 {
		output.WriteString("Top Violations:\n")
		for _, violation := range report.Violations {
			output.WriteString(fmt.Sprintf("  %s: %s (count: %d)\n", violation.Endpoint, violation.Message, violation.Count))
		}
	}

	return []byte(output.String())
}

func formatComplianceReportJUnit(report *validation.ComplianceReport) ([]byte, error) {
	junit := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="API Compliance" tests="%d" failures="%d" errors="0" time="0">`,
		report.TotalRequests, report.InvalidRequests)

	for _, violation := range report.Violations {
		junit += fmt.Sprintf(`
    <testcase name="%s %s" classname="%s">
        <failure message="%s" type="%s">
            %s (occurred %d times)
        </failure>
    </testcase>`,
			violation.Endpoint, violation.Method, violation.Type,
			violation.Message, violation.Type,
			violation.Message, violation.Count)
	}

	junit += "\n</testsuite>"

	return []byte(junit), nil
}
