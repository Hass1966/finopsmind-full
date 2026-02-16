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
