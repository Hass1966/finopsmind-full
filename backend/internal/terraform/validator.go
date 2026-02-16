package terraform

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
	Formatted string           `json:"formatted,omitempty"`
}

type ValidationError struct {
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
}

type Validator struct {
	parser *hclparse.Parser
}

func NewValidator() *Validator {
	return &Validator{parser: hclparse.NewParser()}
}

func (v *Validator) Validate(hclCode string) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationError{}}

	file, diags := v.parser.ParseHCL([]byte(hclCode), "generated.tf")
	
	for _, diag := range diags {
		verr := ValidationError{Message: diag.Detail}
		if diag.Subject != nil {
			verr.Line = diag.Subject.Start.Line
			verr.Column = diag.Subject.Start.Column
		}
		if diag.Severity == hcl.DiagError {
			result.Valid = false
			result.Errors = append(result.Errors, verr)
		} else {
			result.Warnings = append(result.Warnings, verr)
		}
	}

	if result.Valid && file != nil {
		result.Formatted = v.Format(hclCode)
	}

	return result
}

func (v *Validator) Format(hclCode string) string {
	f, diags := hclwrite.ParseConfig([]byte(hclCode), "generated.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return hclCode
	}
	return string(f.Bytes())
}

func (v *Validator) ValidateResourceBlock(hclCode string) *ValidationResult {
	result := v.Validate(hclCode)
	if !result.Valid {
		return result
	}

	hclCode = strings.TrimSpace(hclCode)
	if !strings.Contains(hclCode, "resource ") && 
	   !strings.Contains(hclCode, "data ") &&
	   !strings.Contains(hclCode, "module ") &&
	   !strings.Contains(hclCode, "variable ") &&
	   !strings.Contains(hclCode, "output ") &&
	   !strings.Contains(hclCode, "locals ") &&
	   !strings.Contains(hclCode, "provider ") {
		result.Warnings = append(result.Warnings, ValidationError{
			Message: "HCL does not contain any Terraform blocks",
		})
	}

	return result
}

func (v *Validator) CheckPlaceholders(hclCode string) []string {
	placeholders := []string{}
	patterns := []string{"{{.", "{{ .", "<no value>", "${var."}
	for _, pattern := range patterns {
		if strings.Contains(hclCode, pattern) {
			placeholders = append(placeholders, fmt.Sprintf("Found unsubstituted placeholder: %s", pattern))
		}
	}
	return placeholders
}

func (v *Validator) ValidateAndFormat(hclCode string) (string, error) {
	result := v.Validate(hclCode)
	if !result.Valid {
		errMsgs := []string{}
		for _, err := range result.Errors {
			if err.Line > 0 {
				errMsgs = append(errMsgs, fmt.Sprintf("line %d: %s", err.Line, err.Message))
			} else {
				errMsgs = append(errMsgs, err.Message)
			}
		}
		return "", fmt.Errorf("HCL validation failed: %s", strings.Join(errMsgs, "; "))
	}
	return result.Formatted, nil
}
