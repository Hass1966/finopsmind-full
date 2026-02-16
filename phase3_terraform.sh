#!/bin/bash

# ============================================================================
# FinOpsMind Phase 3: Terraform Code Generation
# ============================================================================
# This script creates all files needed for generating Terraform code
# from cost optimization recommendations.
#
# Run from: ~/Documents/finopsmind-full
# Usage: ./phase3_terraform.sh
# ============================================================================

set -e

echo "============================================"
echo "FinOpsMind Phase 3: Terraform Code Generation"
echo "============================================"
echo ""

# Check we're in the right directory
if [ ! -f "docker-compose.yml" ]; then
    echo "Error: Please run this script from ~/Documents/finopsmind-full"
    exit 1
fi

# ============================================================================
# 1. Create directory structure
# ============================================================================
echo "[1/7] Creating directory structure..."

mkdir -p backend/internal/terraform/templates
mkdir -p frontend/src/components

echo "  ✓ Directories created"

# ============================================================================
# 2. Create backend/internal/terraform/generator.go
# ============================================================================
echo "[2/7] Creating terraform/generator.go..."

cat > backend/internal/terraform/generator.go << 'GOFILE'
package terraform

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// Recommendation represents a cost optimization recommendation
type Recommendation struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	ResourceID      string                 `json:"resource_id"`
	ResourceType    string                 `json:"resource_type"`
	ResourceName    string                 `json:"resource_name"`
	Region          string                 `json:"region"`
	AccountID       string                 `json:"account_id"`
	CurrentValue    string                 `json:"current_value"`
	NewValue        string                 `json:"new_value"`
	CurrentCost     float64                `json:"current_cost"`
	ProjectedCost   float64                `json:"projected_cost"`
	MonthlySavings  float64                `json:"monthly_savings"`
	AnnualSavings   float64                `json:"annual_savings"`
	Confidence      float64                `json:"confidence"`
	Impact          string                 `json:"impact"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// Generator handles Terraform code generation from recommendations
type Generator struct {
	templates map[string]*template.Template
}

// NewGenerator creates a new Terraform generator with loaded templates
func NewGenerator() (*Generator, error) {
	g := &Generator{
		templates: make(map[string]*template.Template),
	}

	templateNames := []string{
		"ec2_rightsize", "ec2_stop", "ec2_terminate",
		"ebs_rightsize", "ebs_gp3_upgrade", "ebs_delete",
		"s3_lifecycle", "s3_intelligent_tiering",
		"rds_rightsize", "rds_stop", "lambda_memory",
		"vpc_endpoint_s3", "vpc_endpoint_dynamodb",
		"eip_release", "snapshot_delete", "cloudwatch_retention",
	}

	for _, name := range templateNames {
		tmpl, err := LoadTemplate(name)
		if err != nil {
			return nil, fmt.Errorf("failed to load template %s: %w", name, err)
		}
		g.templates[name] = tmpl
	}

	return g, nil
}

// TemplateData holds data passed to Terraform templates
type TemplateData struct {
	ResourceID       string
	ResourceName     string
	ResourceType     string
	Region           string
	AccountID        string
	CurrentValue     string
	NewValue         string
	MonthlySavings   float64
	AnnualSavings    float64
	Confidence       float64
	GeneratedAt      string
	RecommendationID string
	InstanceType       string
	NewInstanceType    string
	VolumeType         string
	NewVolumeType      string
	VolumeSize         int
	NewVolumeSize      int
	IOPS               int
	Throughput         int
	BucketName         string
	TransitionDays     int
	ExpirationDays     int
	DBInstanceClass    string
	NewDBInstanceClass string
	MemorySize         int
	NewMemorySize      int
	VPCId              string
	RouteTableIds      []string
	ServiceName        string
	SnapshotID         string
	LogGroupName       string
	RetentionDays      int
}

// Generate creates Terraform HCL code for a recommendation
func (g *Generator) Generate(rec *Recommendation) (string, error) {
	templateName := g.getTemplateName(rec.Type)
	if templateName == "" {
		return "", fmt.Errorf("unsupported recommendation type: %s", rec.Type)
	}

	tmpl, ok := g.templates[templateName]
	if !ok {
		return "", fmt.Errorf("template not found: %s", templateName)
	}

	data := g.buildTemplateData(rec)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) getTemplateName(recType string) string {
	typeMap := map[string]string{
		"ec2_rightsize": "ec2_rightsize", "ec2_rightsizing": "ec2_rightsize",
		"ec2_stop": "ec2_stop", "ec2_stop_idle": "ec2_stop",
		"ec2_terminate": "ec2_terminate", "ec2_terminate_unused": "ec2_terminate",
		"ebs_rightsize": "ebs_rightsize", "ebs_rightsizing": "ebs_rightsize",
		"ebs_gp3_upgrade": "ebs_gp3_upgrade", "ebs_gp2_to_gp3": "ebs_gp3_upgrade",
		"ebs_delete": "ebs_delete", "ebs_delete_unattached": "ebs_delete",
		"s3_lifecycle": "s3_lifecycle", "s3_lifecycle_policy": "s3_lifecycle",
		"s3_intelligent_tiering": "s3_intelligent_tiering", "s3_tiering": "s3_intelligent_tiering",
		"rds_rightsize": "rds_rightsize", "rds_rightsizing": "rds_rightsize",
		"rds_stop": "rds_stop", "rds_stop_idle": "rds_stop",
		"lambda_memory": "lambda_memory", "lambda_rightsizing": "lambda_memory",
		"vpc_endpoint_s3": "vpc_endpoint_s3", "vpc_endpoint_dynamodb": "vpc_endpoint_dynamodb",
		"eip_release": "eip_release", "eip_release_unused": "eip_release",
		"snapshot_delete": "snapshot_delete", "snapshot_delete_old": "snapshot_delete",
		"cloudwatch_retention": "cloudwatch_retention", "cloudwatch_logs": "cloudwatch_retention",
	}
	return typeMap[strings.ToLower(recType)]
}

func (g *Generator) buildTemplateData(rec *Recommendation) *TemplateData {
	data := &TemplateData{
		ResourceID:       rec.ResourceID,
		ResourceName:     rec.ResourceName,
		ResourceType:     rec.ResourceType,
		Region:           rec.Region,
		AccountID:        rec.AccountID,
		CurrentValue:     rec.CurrentValue,
		NewValue:         rec.NewValue,
		MonthlySavings:   rec.MonthlySavings,
		AnnualSavings:    rec.AnnualSavings,
		Confidence:       rec.Confidence,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		RecommendationID: rec.ID,
		TransitionDays:   30,
		ExpirationDays:   365,
		RetentionDays:    30,
		IOPS:             3000,
		Throughput:       125,
	}

	if rec.Metadata != nil {
		if v, ok := rec.Metadata["instance_type"].(string); ok { data.InstanceType = v }
		if v, ok := rec.Metadata["new_instance_type"].(string); ok { data.NewInstanceType = v }
		if v, ok := rec.Metadata["volume_type"].(string); ok { data.VolumeType = v }
		if v, ok := rec.Metadata["new_volume_type"].(string); ok { data.NewVolumeType = v }
		if v, ok := rec.Metadata["volume_size"].(float64); ok { data.VolumeSize = int(v) }
		if v, ok := rec.Metadata["new_volume_size"].(float64); ok { data.NewVolumeSize = int(v) }
		if v, ok := rec.Metadata["iops"].(float64); ok { data.IOPS = int(v) }
		if v, ok := rec.Metadata["throughput"].(float64); ok { data.Throughput = int(v) }
		if v, ok := rec.Metadata["bucket_name"].(string); ok { data.BucketName = v }
		if v, ok := rec.Metadata["transition_days"].(float64); ok { data.TransitionDays = int(v) }
		if v, ok := rec.Metadata["expiration_days"].(float64); ok { data.ExpirationDays = int(v) }
		if v, ok := rec.Metadata["db_instance_class"].(string); ok { data.DBInstanceClass = v }
		if v, ok := rec.Metadata["new_db_instance_class"].(string); ok { data.NewDBInstanceClass = v }
		if v, ok := rec.Metadata["memory_size"].(float64); ok { data.MemorySize = int(v) }
		if v, ok := rec.Metadata["new_memory_size"].(float64); ok { data.NewMemorySize = int(v) }
		if v, ok := rec.Metadata["vpc_id"].(string); ok { data.VPCId = v }
		if v, ok := rec.Metadata["route_table_ids"].([]interface{}); ok {
			for _, rt := range v {
				if s, ok := rt.(string); ok { data.RouteTableIds = append(data.RouteTableIds, s) }
			}
		}
		if v, ok := rec.Metadata["service_name"].(string); ok { data.ServiceName = v }
		if v, ok := rec.Metadata["snapshot_id"].(string); ok { data.SnapshotID = v }
		if v, ok := rec.Metadata["log_group_name"].(string); ok { data.LogGroupName = v }
		if v, ok := rec.Metadata["retention_days"].(float64); ok { data.RetentionDays = int(v) }
	}

	if data.InstanceType == "" { data.InstanceType = rec.CurrentValue }
	if data.NewInstanceType == "" { data.NewInstanceType = rec.NewValue }

	return data
}

func (g *Generator) GetSupportedTypes() []string {
	return []string{
		"ec2_rightsize", "ec2_stop", "ec2_terminate",
		"ebs_rightsize", "ebs_gp3_upgrade", "ebs_delete",
		"s3_lifecycle", "s3_intelligent_tiering",
		"rds_rightsize", "rds_stop", "lambda_memory",
		"vpc_endpoint_s3", "vpc_endpoint_dynamodb",
		"eip_release", "snapshot_delete", "cloudwatch_retention",
	}
}

func (g *Generator) IsSupported(recType string) bool {
	return g.getTemplateName(recType) != ""
}
GOFILE

echo "  ✓ generator.go created"

# ============================================================================
# 3. Create backend/internal/terraform/validator.go
# ============================================================================
echo "[3/7] Creating terraform/validator.go..."

cat > backend/internal/terraform/validator.go << 'GOFILE'
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
GOFILE

echo "  ✓ validator.go created"

# ============================================================================
# 4. Create backend/internal/terraform/templates.go
# ============================================================================
echo "[4/7] Creating terraform/templates.go..."

cat > backend/internal/terraform/templates.go << 'GOFILE'
package terraform

import (
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

func LoadTemplate(name string) (*template.Template, error) {
	filename := fmt.Sprintf("templates/%s.tf.tmpl", name)
	content, err := templateFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	return tmpl, nil
}

func ListTemplates() ([]string, error) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if len(name) > 8 && name[len(name)-8:] == ".tf.tmpl" {
				names = append(names, name[:len(name)-8])
			}
		}
	}
	return names, nil
}

func GetTemplateContent(name string) (string, error) {
	filename := fmt.Sprintf("templates/%s.tf.tmpl", name)
	content, err := templateFS.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", name, err)
	}
	return string(content), nil
}
GOFILE

echo "  ✓ templates.go created"

# ============================================================================
# 5. Create Terraform templates
# ============================================================================
echo "[5/7] Creating Terraform templates..."

# EC2 Rightsize Template
cat > backend/internal/terraform/templates/ec2_rightsize.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EC2 Instance Rightsizing
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Generated at: {{.GeneratedAt}}
# Current: {{.InstanceType}} -> Recommended: {{.NewInstanceType}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}} | Annual: ${{printf "%.2f" .AnnualSavings}}
# Confidence: {{printf "%.0f" .Confidence}}%
#
# IMPORTANT: Requires instance stop/start. Schedule during maintenance.
# terraform import aws_instance.{{.ResourceName}} {{.ResourceID}}
# ============================================================================

resource "aws_instance" "{{.ResourceName}}" {
  instance_type = "{{.NewInstanceType}}"

  tags = {
    Name                = "{{.ResourceName}}"
    ManagedBy           = "terraform"
    FinOpsMind          = "rightsized"
    PreviousInstanceType = "{{.InstanceType}}"
  }

  lifecycle {
    prevent_destroy = false
  }
}

output "{{.ResourceName}}_instance_type" {
  value = aws_instance.{{.ResourceName}}.instance_type
}
TMPLFILE

# EC2 Stop Template
cat > backend/internal/terraform/templates/ec2_stop.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EC2 Instance Stop (Idle Resource)
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Resource: {{.ResourceID}} ({{.ResourceName}})
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# Note: Stopped instances still incur EBS and EIP charges.
# ============================================================================

resource "null_resource" "stop_{{.ResourceName}}" {
  provisioner "local-exec" {
    command = "aws ec2 stop-instances --instance-ids {{.ResourceID}} --region {{.Region}}"
  }
  triggers = { instance_id = "{{.ResourceID}}" }
}
TMPLFILE

# EC2 Terminate Template
cat > backend/internal/terraform/templates/ec2_terminate.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EC2 Instance Termination (Unused Resource)
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Resource: {{.ResourceID}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# WARNING: IRREVERSIBLE! Backup data and create AMI if needed.
# terraform import aws_instance.{{.ResourceName}}_to_terminate {{.ResourceID}}
# terraform destroy -target=aws_instance.{{.ResourceName}}_to_terminate
# ============================================================================

resource "aws_instance" "{{.ResourceName}}_to_terminate" {
  lifecycle { prevent_destroy = false }
}
TMPLFILE

# EBS Rightsize Template
cat > backend/internal/terraform/templates/ebs_rightsize.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EBS Volume Rightsizing
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Current: {{.VolumeSize}} GB -> Recommended: {{.NewVolumeSize}} GB
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# Note: EBS can only increase, not decrease. Downsizing requires migration.
# terraform import aws_ebs_volume.{{.ResourceName}} {{.ResourceID}}
# ============================================================================

resource "aws_ebs_volume" "{{.ResourceName}}" {
  availability_zone = "{{.Region}}a"
  size              = {{.NewVolumeSize}}
  type              = "{{.VolumeType}}"

  tags = {
    Name         = "{{.ResourceName}}"
    ManagedBy    = "terraform"
    FinOpsMind   = "rightsized"
    PreviousSize = "{{.VolumeSize}}"
  }
}
TMPLFILE

# EBS GP3 Upgrade Template
cat > backend/internal/terraform/templates/ebs_gp3_upgrade.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EBS Volume Upgrade: GP2 to GP3
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Current: {{.VolumeType}} -> New: gp3
# Size: {{.VolumeSize}} GB | IOPS: {{.IOPS}} | Throughput: {{.Throughput}} MB/s
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# GP3 is 20% cheaper with independent IOPS/throughput. Non-disruptive migration.
# terraform import aws_ebs_volume.{{.ResourceName}} {{.ResourceID}}
# ============================================================================

resource "aws_ebs_volume" "{{.ResourceName}}" {
  availability_zone = "{{.Region}}a"
  size              = {{.VolumeSize}}
  type              = "gp3"
  iops              = {{.IOPS}}
  throughput        = {{.Throughput}}

  tags = {
    Name         = "{{.ResourceName}}"
    ManagedBy    = "terraform"
    FinOpsMind   = "gp3-upgraded"
    PreviousType = "{{.VolumeType}}"
  }
}
TMPLFILE

# EBS Delete Template
cat > backend/internal/terraform/templates/ebs_delete.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EBS Volume Deletion (Unattached)
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Resource: {{.ResourceID}} ({{.VolumeSize}} GB {{.VolumeType}})
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# WARNING: IRREVERSIBLE! Create snapshot first if needed.
# ============================================================================

resource "aws_ebs_snapshot" "{{.ResourceName}}_final" {
  volume_id   = "{{.ResourceID}}"
  description = "Final snapshot before deletion - FinOpsMind"
  tags = { Name = "{{.ResourceName}}-final-snapshot", SourceVolume = "{{.ResourceID}}" }
}

resource "aws_ebs_volume" "{{.ResourceName}}_to_delete" {
  availability_zone = "{{.Region}}a"
  size              = {{.VolumeSize}}
  type              = "{{.VolumeType}}"
  lifecycle { prevent_destroy = false }
}
TMPLFILE

# S3 Lifecycle Template
cat > backend/internal/terraform/templates/s3_lifecycle.tf.tmpl << 'TMPLFILE'
# ============================================================================
# S3 Bucket Lifecycle Policy
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Bucket: {{.BucketName}}
# Transitions: Standard-IA ({{.TransitionDays}}d), Glacier (90d)
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
# ============================================================================

resource "aws_s3_bucket_lifecycle_configuration" "{{.ResourceName}}_lifecycle" {
  bucket = "{{.BucketName}}"

  rule {
    id     = "finopsmind-optimization"
    status = "Enabled"
    filter { prefix = "" }

    transition {
      days          = {{.TransitionDays}}
      storage_class = "STANDARD_IA"
    }

    transition {
      days          = 90
      storage_class = "GLACIER"
    }

    abort_incomplete_multipart_upload { days_after_initiation = 7 }
  }

  rule {
    id     = "cleanup-old-versions"
    status = "Enabled"
    filter { prefix = "" }

    noncurrent_version_transition {
      noncurrent_days = 30
      storage_class   = "GLACIER"
    }

    noncurrent_version_expiration { noncurrent_days = 90 }
  }
}
TMPLFILE

# S3 Intelligent Tiering Template
cat > backend/internal/terraform/templates/s3_intelligent_tiering.tf.tmpl << 'TMPLFILE'
# ============================================================================
# S3 Intelligent Tiering Configuration
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Bucket: {{.BucketName}}
# Auto-tiers based on access patterns. No retrieval fees.
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
# ============================================================================

resource "aws_s3_bucket_intelligent_tiering_configuration" "{{.ResourceName}}_tiering" {
  bucket = "{{.BucketName}}"
  name   = "finopsmind-intelligent-tiering"

  tiering {
    access_tier = "ARCHIVE_ACCESS"
    days        = 90
  }

  tiering {
    access_tier = "DEEP_ARCHIVE_ACCESS"
    days        = 180
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "{{.ResourceName}}_to_intelligent_tiering" {
  bucket = "{{.BucketName}}"

  rule {
    id     = "move-to-intelligent-tiering"
    status = "Enabled"
    filter { prefix = "" }

    transition {
      days          = 0
      storage_class = "INTELLIGENT_TIERING"
    }
  }
}
TMPLFILE

# RDS Rightsize Template
cat > backend/internal/terraform/templates/rds_rightsize.tf.tmpl << 'TMPLFILE'
# ============================================================================
# RDS Instance Rightsizing
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Current: {{.DBInstanceClass}} -> Recommended: {{.NewDBInstanceClass}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# WARNING: Brief outage during modification. Use maintenance window.
# terraform import aws_db_instance.{{.ResourceName}} {{.ResourceID}}
# ============================================================================

resource "aws_db_instance" "{{.ResourceName}}" {
  identifier     = "{{.ResourceName}}"
  instance_class = "{{.NewDBInstanceClass}}"
  apply_immediately = false

  tags = {
    Name                  = "{{.ResourceName}}"
    ManagedBy             = "terraform"
    FinOpsMind            = "rightsized"
    PreviousInstanceClass = "{{.DBInstanceClass}}"
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = [password]
  }
}
TMPLFILE

# RDS Stop Template
cat > backend/internal/terraform/templates/rds_stop.tf.tmpl << 'TMPLFILE'
# ============================================================================
# RDS Instance Stop (Idle Database)
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Resource: {{.ResourceID}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# Note: Still incurs storage charges. Auto-starts after 7 days.
# ============================================================================

resource "null_resource" "stop_rds_{{.ResourceName}}" {
  provisioner "local-exec" {
    command = "aws rds stop-db-instance --db-instance-identifier {{.ResourceID}} --region {{.Region}}"
  }
  triggers = { instance_id = "{{.ResourceID}}" }
}
TMPLFILE

# Lambda Memory Template
cat > backend/internal/terraform/templates/lambda_memory.tf.tmpl << 'TMPLFILE'
# ============================================================================
# Lambda Function Memory Rightsizing
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Current: {{.MemorySize}} MB -> Recommended: {{.NewMemorySize}} MB
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# Note: Memory affects CPU allocation. More memory can mean faster execution.
# terraform import aws_lambda_function.{{.ResourceName}} {{.ResourceID}}
# ============================================================================

resource "aws_lambda_function" "{{.ResourceName}}" {
  function_name = "{{.ResourceName}}"
  memory_size   = {{.NewMemorySize}}

  tags = {
    ManagedBy      = "terraform"
    FinOpsMind     = "memory-optimized"
    PreviousMemory = "{{.MemorySize}}"
  }

  lifecycle {
    ignore_changes = [filename, source_code_hash, last_modified]
  }
}
TMPLFILE

# VPC Endpoint S3 Template
cat > backend/internal/terraform/templates/vpc_endpoint_s3.tf.tmpl << 'TMPLFILE'
# ============================================================================
# VPC Gateway Endpoint for S3
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# VPC: {{.VPCId}} | Region: {{.Region}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# Gateway endpoints are FREE. Eliminates S3 data transfer charges.
# ============================================================================

resource "aws_vpc_endpoint" "s3_gateway" {
  vpc_id            = "{{.VPCId}}"
  service_name      = "com.amazonaws.{{.Region}}.s3"
  vpc_endpoint_type = "Gateway"

  route_table_ids = [
    {{- range $i, $rt := .RouteTableIds}}
    {{if $i}},{{end}}"{{$rt}}"
    {{- end}}
  ]

  tags = {
    Name       = "s3-gateway-endpoint"
    ManagedBy  = "terraform"
    FinOpsMind = "cost-optimization"
  }
}

output "s3_endpoint_id" {
  value = aws_vpc_endpoint.s3_gateway.id
}
TMPLFILE

# VPC Endpoint DynamoDB Template
cat > backend/internal/terraform/templates/vpc_endpoint_dynamodb.tf.tmpl << 'TMPLFILE'
# ============================================================================
# VPC Gateway Endpoint for DynamoDB
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# VPC: {{.VPCId}} | Region: {{.Region}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
# ============================================================================

resource "aws_vpc_endpoint" "dynamodb_gateway" {
  vpc_id            = "{{.VPCId}}"
  service_name      = "com.amazonaws.{{.Region}}.dynamodb"
  vpc_endpoint_type = "Gateway"

  route_table_ids = [
    {{- range $i, $rt := .RouteTableIds}}
    {{if $i}},{{end}}"{{$rt}}"
    {{- end}}
  ]

  tags = {
    Name       = "dynamodb-gateway-endpoint"
    ManagedBy  = "terraform"
    FinOpsMind = "cost-optimization"
  }
}
TMPLFILE

# EIP Release Template
cat > backend/internal/terraform/templates/eip_release.tf.tmpl << 'TMPLFILE'
# ============================================================================
# Elastic IP Release (Unused)
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Resource: {{.ResourceID}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# WARNING: Cannot reclaim specific IP after release. Check DNS dependencies.
# terraform import aws_eip.{{.ResourceName}}_to_release {{.ResourceID}}
# terraform destroy -target=aws_eip.{{.ResourceName}}_to_release
# ============================================================================

resource "aws_eip" "{{.ResourceName}}_to_release" {
  tags = {
    Name       = "{{.ResourceName}}-to-release"
    FinOpsMind = "scheduled-for-release"
  }
  lifecycle { prevent_destroy = false }
}
TMPLFILE

# Snapshot Delete Template
cat > backend/internal/terraform/templates/snapshot_delete.tf.tmpl << 'TMPLFILE'
# ============================================================================
# EBS Snapshot Deletion (Old/Unused)
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Snapshot: {{.SnapshotID}}
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# WARNING: IRREVERSIBLE! Ensure no AMIs depend on this snapshot.
# ============================================================================

resource "null_resource" "delete_snapshot_{{.ResourceName}}" {
  provisioner "local-exec" {
    command = <<-EOT
      SNAPSHOT_STATE=$(aws ec2 describe-snapshots --snapshot-ids {{.SnapshotID}} --region {{.Region}} --query 'Snapshots[0].State' --output text 2>/dev/null || echo "not-found")
      if [ "$SNAPSHOT_STATE" = "completed" ]; then
        aws ec2 delete-snapshot --snapshot-id {{.SnapshotID}} --region {{.Region}}
        echo "Snapshot deleted"
      else
        echo "Snapshot not found or not completed"
      fi
    EOT
  }
  triggers = { snapshot_id = "{{.SnapshotID}}" }
}
TMPLFILE

# CloudWatch Retention Template
cat > backend/internal/terraform/templates/cloudwatch_retention.tf.tmpl << 'TMPLFILE'
# ============================================================================
# CloudWatch Logs Retention Policy
# ============================================================================
# Recommendation ID: {{.RecommendationID}}
# Log Group: {{.LogGroupName}}
# Retention: {{.RetentionDays}} days
# Monthly savings: ${{printf "%.2f" .MonthlySavings}}
#
# Note: Logs older than retention period are permanently deleted.
# Export to S3 first if archival is needed.
# ============================================================================

resource "aws_cloudwatch_log_group" "{{.ResourceName}}" {
  name              = "{{.LogGroupName}}"
  retention_in_days = {{.RetentionDays}}

  tags = {
    ManagedBy     = "terraform"
    FinOpsMind    = "retention-optimized"
  }
}
TMPLFILE

echo "  ✓ 16 Terraform templates created"

# ============================================================================
# 6. Create backend/internal/handler/terraform.go
# ============================================================================
echo "[6/7] Creating handler/terraform.go..."

cat > backend/internal/handler/terraform.go << 'GOFILE'
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"finopsmind/internal/terraform"
)

type TerraformHandler struct {
	generator *terraform.Generator
	validator *terraform.Validator
}

func NewTerraformHandler() (*TerraformHandler, error) {
	gen, err := terraform.NewGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}
	return &TerraformHandler{generator: gen, validator: terraform.NewValidator()}, nil
}

func (h *TerraformHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/recommendations/{id}/terraform", h.GetTerraform).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/recommendations/{id}/terraform/download", h.DownloadTerraform).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/terraform/supported-types", h.GetSupportedTypes).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/terraform/validate", h.ValidateHCL).Methods("POST", "OPTIONS")
}

type TerraformResponse struct {
	Success    bool                        `json:"success"`
	HCL        string                      `json:"hcl,omitempty"`
	Formatted  string                      `json:"formatted,omitempty"`
	Validation *terraform.ValidationResult `json:"validation,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Metadata   TerraformMetadata           `json:"metadata"`
}

type TerraformMetadata struct {
	RecommendationType string   `json:"recommendation_type"`
	TemplateName       string   `json:"template_name"`
	ResourceID         string   `json:"resource_id"`
	ImportCommand      string   `json:"import_command,omitempty"`
	ApplyWarnings      []string `json:"apply_warnings,omitempty"`
}

func (h *TerraformHandler) GetTerraform(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }

	vars := mux.Vars(r)
	recommendationID := vars["id"]
	if recommendationID == "" { h.sendError(w, http.StatusBadRequest, "recommendation ID required"); return }

	rec := h.getMockRecommendation(recommendationID)
	if rec == nil { h.sendError(w, http.StatusNotFound, "recommendation not found"); return }

	if !h.generator.IsSupported(rec.Type) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("unsupported type: %s", rec.Type)); return
	}

	hcl, err := h.generator.Generate(rec)
	if err != nil { h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("generation failed: %v", err)); return }

	validation := h.validator.ValidateResourceBlock(hcl)

	response := TerraformResponse{
		Success: true, HCL: hcl, Formatted: validation.Formatted, Validation: validation,
		Metadata: TerraformMetadata{
			RecommendationType: rec.Type,
			TemplateName:       strings.ToLower(rec.Type),
			ResourceID:         rec.ResourceID,
			ImportCommand:      h.getImportCommand(rec),
			ApplyWarnings:      h.getApplyWarnings(rec),
		},
	}
	h.sendJSON(w, http.StatusOK, response)
}

func (h *TerraformHandler) DownloadTerraform(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }

	vars := mux.Vars(r)
	rec := h.getMockRecommendation(vars["id"])
	if rec == nil { h.sendError(w, http.StatusNotFound, "not found"); return }

	hcl, _ := h.generator.Generate(rec)
	formatted, err := h.validator.ValidateAndFormat(hcl)
	if err != nil { formatted = hcl }

	filename := fmt.Sprintf("%s_%s.tf", rec.Type, rec.ResourceName)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Write([]byte(formatted))
}

func (h *TerraformHandler) GetSupportedTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }
	types := h.generator.GetSupportedTypes()
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"success": true, "supported_types": types, "count": len(types)})
}

func (h *TerraformHandler) ValidateHCL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }

	var req struct { HCL string `json:"hcl"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { h.sendError(w, http.StatusBadRequest, "invalid body"); return }
	if req.HCL == "" { h.sendError(w, http.StatusBadRequest, "HCL required"); return }

	validation := h.validator.ValidateResourceBlock(req.HCL)
	placeholders := h.validator.CheckPlaceholders(req.HCL)
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"success": true, "validation": validation, "placeholders": placeholders})
}

func (h *TerraformHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TerraformHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, TerraformResponse{Success: false, Error: message})
}

func (h *TerraformHandler) getImportCommand(rec *terraform.Recommendation) string {
	switch {
	case strings.HasPrefix(rec.Type, "ec2"):
		return fmt.Sprintf("terraform import aws_instance.%s %s", rec.ResourceName, rec.ResourceID)
	case strings.HasPrefix(rec.Type, "ebs"):
		return fmt.Sprintf("terraform import aws_ebs_volume.%s %s", rec.ResourceName, rec.ResourceID)
	case strings.HasPrefix(rec.Type, "rds"):
		return fmt.Sprintf("terraform import aws_db_instance.%s %s", rec.ResourceName, rec.ResourceID)
	default:
		return ""
	}
}

func (h *TerraformHandler) getApplyWarnings(rec *terraform.Recommendation) []string {
	warnings := []string{}
	switch rec.Type {
	case "ec2_rightsize":
		warnings = append(warnings, "Instance will be stopped and started")
	case "ec2_terminate", "ebs_delete":
		warnings = append(warnings, "DESTRUCTIVE: Resource will be permanently deleted")
	case "rds_rightsize":
		warnings = append(warnings, "Brief downtime during modification")
	}
	return warnings
}

func (h *TerraformHandler) getMockRecommendation(id string) *terraform.Recommendation {
	mocks := map[string]*terraform.Recommendation{
		"rec-001": {
			ID: "rec-001", Type: "ec2_rightsize", ResourceID: "i-0123456789abcdef0",
			ResourceType: "EC2", ResourceName: "web_server_1", Region: "us-east-1",
			AccountID: "123456789012", CurrentValue: "m5.xlarge", NewValue: "m5.large",
			CurrentCost: 140.16, ProjectedCost: 70.08, MonthlySavings: 70.08, AnnualSavings: 840.96,
			Confidence: 92.5, Impact: "medium",
			Metadata: map[string]interface{}{"instance_type": "m5.xlarge", "new_instance_type": "m5.large"},
		},
		"rec-002": {
			ID: "rec-002", Type: "ebs_gp3_upgrade", ResourceID: "vol-0123456789abcdef0",
			ResourceType: "EBS", ResourceName: "data_volume_1", Region: "us-east-1",
			CurrentValue: "gp2", NewValue: "gp3", MonthlySavings: 20.00, AnnualSavings: 240.00, Confidence: 98.0,
			Metadata: map[string]interface{}{"volume_type": "gp2", "volume_size": float64(500), "iops": float64(3000), "throughput": float64(125)},
		},
		"rec-003": {
			ID: "rec-003", Type: "s3_lifecycle", ResourceID: "my-data-bucket",
			ResourceType: "S3", ResourceName: "data_bucket", Region: "us-east-1",
			MonthlySavings: 300.00, AnnualSavings: 3600.00, Confidence: 85.0,
			Metadata: map[string]interface{}{"bucket_name": "my-data-bucket", "transition_days": float64(30)},
		},
	}
	return mocks[id]
}
GOFILE

echo "  ✓ handler/terraform.go created"

# ============================================================================
# 7. Create frontend components
# ============================================================================
echo "[7/7] Creating frontend components..."

cat > frontend/src/components/TerraformViewer.tsx << 'TSXFILE'
import React, { useState, useEffect, useCallback } from 'react';

interface ValidationResult {
  valid: boolean;
  errors?: Array<{ line?: number; message: string }>;
  warnings?: Array<{ line?: number; message: string }>;
  formatted?: string;
}

interface TerraformMetadata {
  recommendation_type: string;
  template_name: string;
  resource_id: string;
  import_command?: string;
  apply_warnings?: string[];
}

interface TerraformResponse {
  success: boolean;
  hcl?: string;
  formatted?: string;
  validation?: ValidationResult;
  error?: string;
  metadata: TerraformMetadata;
}

interface TerraformViewerProps {
  recommendationId: string;
  onClose?: () => void;
  apiBaseUrl?: string;
}

const highlightHCL = (code: string): string => {
  let h = code
    .replace(/\b(resource|data|variable|output|module|provider|terraform|locals|for_each|count|depends_on|lifecycle|provisioner)\b/g, '<span class="keyword">$1</span>')
    .replace(/"([^"\\]|\\.)*"/g, '<span class="string">$&</span>')
    .replace(/(#.*$|\/\/.*$)/gm, '<span class="comment">$1</span>')
    .replace(/\b(\d+\.?\d*)\b/g, '<span class="number">$1</span>')
    .replace(/\b(true|false|null)\b/g, '<span class="boolean">$1</span>')
    .replace(/(\w+)(\s*=)/g, '<span class="attribute">$1</span>$2');
  return h;
};

const TerraformViewer: React.FC<TerraformViewerProps> = ({ recommendationId, onClose, apiBaseUrl = 'http://localhost:8080' }) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TerraformResponse | null>(null);
  const [copied, setCopied] = useState(false);
  const [showFormatted, setShowFormatted] = useState(true);

  const fetchTerraform = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/recommendations/${recommendationId}/terraform`);
      const result: TerraformResponse = await response.json();
      if (!result.success) setError(result.error || 'Failed to generate');
      else setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch');
    } finally {
      setLoading(false);
    }
  }, [recommendationId, apiBaseUrl]);

  useEffect(() => { fetchTerraform(); }, [fetchTerraform]);

  const handleCopy = async () => {
    if (!data) return;
    const code = showFormatted && data.formatted ? data.formatted : data.hcl;
    if (!code) return;
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) { console.error('Copy failed:', err); }
  };

  const handleDownload = () => {
    window.open(`${apiBaseUrl}/api/v1/recommendations/${recommendationId}/terraform/download`, '_blank');
  };

  const displayCode = showFormatted && data?.formatted ? data.formatted : data?.hcl || '';
  const codeWithLines = displayCode.split('\n').map((line, i) => {
    const num = (i + 1).toString().padStart(3, ' ');
    return `<span class="line-number">${num}</span> ${highlightHCL(line)}`;
  }).join('\n');

  return (
    <div style={{ fontFamily: 'system-ui', background: '#1e1e1e', borderRadius: '8px', overflow: 'hidden', boxShadow: '0 4px 6px rgba(0,0,0,0.3)' }}>
      <style>{`
        .tf-code .keyword { color: #569cd6; }
        .tf-code .string { color: #ce9178; }
        .tf-code .comment { color: #6a9955; font-style: italic; }
        .tf-code .number { color: #b5cea8; }
        .tf-code .boolean { color: #569cd6; }
        .tf-code .attribute { color: #9cdcfe; }
        .tf-code .line-number { color: #858585; user-select: none; margin-right: 16px; }
      `}</style>

      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 16px', background: '#252526', borderBottom: '1px solid #3c3c3c' }}>
        <div style={{ color: '#ccc', fontSize: '14px', fontWeight: 500, display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ color: '#7b61ff' }}>⬡</span> Terraform Configuration
        </div>
        {onClose && <button onClick={onClose} style={{ padding: '6px 12px', borderRadius: '4px', border: 'none', background: '#3c3c3c', color: '#ccc', cursor: 'pointer' }}>Close</button>}
      </div>

      {loading && <div style={{ padding: '40px', textAlign: 'center', color: '#9cdcfe' }}><p>Generating Terraform code...</p></div>}
      {error && <div style={{ padding: '40px', textAlign: 'center', color: '#f14c4c' }}><p>⚠️ {error}</p><button onClick={fetchTerraform} style={{ padding: '6px 12px', background: '#0e639c', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>Retry</button></div>}

      {data && !loading && !error && (
        <>
          {/* Toolbar */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 16px', background: '#2d2d2d', borderBottom: '1px solid #3c3c3c' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#9cdcfe', fontSize: '12px' }}>
              <input type="checkbox" checked={showFormatted} onChange={(e) => setShowFormatted(e.target.checked)} />
              Show formatted
            </label>
            <div style={{ display: 'flex', gap: '8px' }}>
              {data.validation && <span style={{ fontSize: '12px', color: data.validation.valid ? '#4ec9b0' : '#f14c4c' }}>{data.validation.valid ? '✓ Valid' : '✗ Invalid'}</span>}
              <button onClick={handleCopy} style={{ padding: '6px 12px', borderRadius: '4px', border: 'none', background: copied ? '#28a745' : '#3c3c3c', color: copied ? 'white' : '#ccc', cursor: 'pointer', fontSize: '12px' }}>{copied ? '✓ Copied!' : '📋 Copy'}</button>
              <button onClick={handleDownload} style={{ padding: '6px 12px', borderRadius: '4px', border: 'none', background: '#0e639c', color: 'white', cursor: 'pointer', fontSize: '12px' }}>⬇️ Download .tf</button>
            </div>
          </div>

          {/* Code */}
          <div style={{ maxHeight: '500px', overflow: 'auto' }}>
            <pre className="tf-code" style={{ margin: 0, padding: '16px', fontFamily: "'Fira Code', Consolas, monospace", fontSize: '13px', lineHeight: 1.5, color: '#d4d4d4', background: '#1e1e1e', whiteSpace: 'pre', overflowX: 'auto' }} dangerouslySetInnerHTML={{ __html: codeWithLines }} />
          </div>

          {/* Metadata */}
          <div style={{ padding: '12px 16px', background: '#252526', borderTop: '1px solid #3c3c3c' }}>
            <div style={{ color: '#9cdcfe', fontSize: '12px', fontWeight: 500, marginBottom: '8px' }}>Configuration Details</div>
            <div style={{ fontSize: '12px', color: '#ccc' }}>
              <div><span style={{ color: '#858585', marginRight: '8px' }}>Type:</span><span style={{ color: '#4ec9b0', fontFamily: 'monospace' }}>{data.metadata.recommendation_type}</span></div>
              <div><span style={{ color: '#858585', marginRight: '8px' }}>Resource:</span><span style={{ color: '#4ec9b0', fontFamily: 'monospace' }}>{data.metadata.resource_id}</span></div>
              {data.metadata.import_command && <div><span style={{ color: '#858585', marginRight: '8px' }}>Import:</span><span style={{ color: '#4ec9b0', fontFamily: 'monospace' }}>{data.metadata.import_command}</span></div>}
            </div>
            {data.metadata.apply_warnings && data.metadata.apply_warnings.length > 0 && (
              <div style={{ marginTop: '12px', padding: '8px 12px', background: 'rgba(255,200,0,0.1)', borderLeft: '3px solid #cca700', borderRadius: '0 4px 4px 0' }}>
                <div style={{ color: '#cca700', fontSize: '12px', fontWeight: 500 }}>⚠️ Warnings</div>
                {data.metadata.apply_warnings.map((w, i) => <div key={i} style={{ color: '#e8c76b', fontSize: '11px', marginLeft: '8px' }}>• {w}</div>)}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
};

export default TerraformViewer;
TSXFILE

echo "  ✓ TerraformViewer.tsx created"

# TerraformButton component for integration
cat > frontend/src/components/TerraformButton.tsx << 'TSXFILE'
import React, { useState } from 'react';
import TerraformViewer from './TerraformViewer';

interface Recommendation {
  id: string;
  type: string;
  resourceId?: string;
  resourceName?: string;
  monthlySavings?: number;
}

interface TerraformButtonProps {
  recommendation: Recommendation;
}

const SUPPORTED_TYPES = [
  'ec2_rightsize', 'ec2_stop', 'ec2_terminate',
  'ebs_rightsize', 'ebs_gp3_upgrade', 'ebs_delete',
  's3_lifecycle', 's3_intelligent_tiering',
  'rds_rightsize', 'rds_stop', 'lambda_memory',
  'vpc_endpoint_s3', 'vpc_endpoint_dynamodb',
  'eip_release', 'snapshot_delete', 'cloudwatch_retention'
];

export const TerraformButton: React.FC<TerraformButtonProps> = ({ recommendation }) => {
  const [showViewer, setShowViewer] = useState(false);

  const isSupported = SUPPORTED_TYPES.some(t => 
    recommendation.type.toLowerCase().includes(t) || t.includes(recommendation.type.toLowerCase())
  );

  if (!isSupported) return null;

  return (
    <>
      <button
        onClick={() => setShowViewer(true)}
        style={{
          padding: '6px 12px',
          background: '#7b61ff',
          color: 'white',
          border: 'none',
          borderRadius: '4px',
          fontSize: '12px',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}
      >
        ⬡ View Terraform
      </button>

      {showViewer && (
        <div
          style={{
            position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
            background: 'rgba(0, 0, 0, 0.75)',
            display: 'flex', justifyContent: 'center', alignItems: 'center',
            zIndex: 1000, padding: '20px',
          }}
          onClick={() => setShowViewer(false)}
        >
          <div style={{ width: '100%', maxWidth: '900px', maxHeight: '90vh', overflow: 'auto' }} onClick={(e) => e.stopPropagation()}>
            <TerraformViewer recommendationId={recommendation.id} onClose={() => setShowViewer(false)} />
          </div>
        </div>
      )}
    </>
  );
};

export default TerraformButton;
TSXFILE

echo "  ✓ TerraformButton.tsx created"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "============================================"
echo "Phase 3 Setup Complete!"
echo "============================================"
echo ""
echo "Files created:"
echo "  Backend:"
echo "    - backend/internal/terraform/generator.go"
echo "    - backend/internal/terraform/validator.go"
echo "    - backend/internal/terraform/templates.go"
echo "    - backend/internal/terraform/templates/*.tf.tmpl (16 templates)"
echo "    - backend/internal/handler/terraform.go"
echo ""
echo "  Frontend:"
echo "    - frontend/src/components/TerraformViewer.tsx"
echo "    - frontend/src/components/TerraformButton.tsx"
echo ""
echo "Next steps:"
echo "  1. Add HCL library to go.mod:"
echo "     cd backend && go get github.com/hashicorp/hcl/v2"
echo ""
echo "  2. Register the Terraform handler in your main.go:"
echo "     tfHandler, _ := handler.NewTerraformHandler()"
echo "     tfHandler.RegisterRoutes(router)"
echo ""
echo "  3. Import TerraformButton in your recommendations page:"
echo "     import { TerraformButton } from './components/TerraformButton';"
echo ""
echo "  4. Rebuild and restart:"
echo "     docker compose down && docker compose up --build -d"
echo ""
echo "API Endpoints:"
echo "  GET  /api/v1/recommendations/:id/terraform          - Get Terraform code"
echo "  GET  /api/v1/recommendations/:id/terraform/download - Download .tf file"
echo "  GET  /api/v1/terraform/supported-types              - List supported types"
echo "  POST /api/v1/terraform/validate                     - Validate HCL code"
echo ""
echo "Test with:"
echo "  curl http://localhost:8080/api/v1/recommendations/rec-001/terraform"
echo ""
echo "============================================"
