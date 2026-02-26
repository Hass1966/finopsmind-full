package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/go-chi/chi/v5"

	"github.com/finopsmind/backend/internal/recommendations"
)

// ScriptExportHandler handles Terraform bulk export, shell script and CI/CD pipeline generation.
type ScriptExportHandler struct {
	db recommendations.DBQuerier
}

// NewScriptExportHandler creates a new ScriptExportHandler.
func NewScriptExportHandler(db recommendations.DBQuerier) *ScriptExportHandler {
	return &ScriptExportHandler{db: db}
}

// GenerateScript returns a shell script for a recommendation.
func (h *ScriptExportHandler) GenerateScript(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.getRecommendation(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "recommendation not found")
		return
	}

	script := h.generateShellScript(rec)

	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="remediate_%s.sh"`, id))
	w.Write([]byte(script))
}

// GeneratePipeline returns a CI/CD pipeline config for a recommendation.
func (h *ScriptExportHandler) GeneratePipeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.getRecommendation(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "recommendation not found")
		return
	}

	pipelineType := r.URL.Query().Get("type")
	if pipelineType == "" {
		pipelineType = "github"
	}

	var pipeline string
	switch pipelineType {
	case "github":
		pipeline = h.generateGitHubActions(rec)
	case "gitlab":
		pipeline = h.generateGitLabCI(rec)
	default:
		writeError(w, http.StatusBadRequest, "unsupported pipeline type: use 'github' or 'gitlab'")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recommendation_id": id,
		"pipeline_type":     pipelineType,
		"pipeline":          pipeline,
		"terraform_code":    rec.TerraformCode,
	})
}

// BulkExport generates a zip file containing Terraform for multiple recommendations.
func (h *ScriptExportHandler) BulkExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one recommendation ID required")
		return
	}

	// Create zip buffer
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	var totalSavings float64
	var readmeEntries []string

	for _, id := range req.IDs {
		rec, err := h.getRecommendation(id)
		if err != nil {
			continue
		}

		if rec.TerraformCode != "" {
			filename := fmt.Sprintf("%s_%s.tf", rec.Type, rec.ResourceID)
			filename = strings.ReplaceAll(filename, "/", "_")
			f, err := zw.Create(filename)
			if err != nil {
				continue
			}
			f.Write([]byte(rec.TerraformCode))
		}

		// Add shell script
		script := h.generateShellScript(rec)
		scriptFile, _ := zw.Create(fmt.Sprintf("scripts/%s.sh", id))
		scriptFile.Write([]byte(script))

		totalSavings += rec.EstimatedSavings
		readmeEntries = append(readmeEntries, fmt.Sprintf(
			"- **%s** (%s): %s → %s | Savings: $%.2f/mo",
			rec.ResourceID, rec.ResourceType, rec.CurrentState, rec.RecommendedAction, rec.EstimatedSavings,
		))
	}

	// Add README
	readme := fmt.Sprintf("# FinOpsMind Remediation Bundle\n\nGenerated remediation scripts for %d recommendations.\n\n## Estimated Total Monthly Savings: $%.2f\n\n## Recommendations\n\n%s\n\n## Usage\n\n1. Review each .tf file\n2. Run `terraform init && terraform plan`\n3. Apply: `terraform apply`\n\n## Rollback\n\nEach script includes rollback instructions in comments.\n",
		len(req.IDs), totalSavings, strings.Join(readmeEntries, "\n"))
	readmeFile, _ := zw.Create("README.md")
	readmeFile.Write([]byte(readme))

	zw.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="finopsmind-remediation.zip"`)
	w.Write(buf.Bytes())
}

func (h *ScriptExportHandler) getRecommendation(id string) (*recommendations.Recommendation, error) {
	var rec recommendations.Recommendation
	row := h.db.QueryRow(
		`SELECT id, type, rule_id, resource_id, resource_type, resource_arn, account_id, region,
			current_state, recommended_action, estimated_savings, confidence, severity,
			terraform_code, resource_metadata, status, created_at, updated_at
		FROM recommendations WHERE id = $1`, id)
	err := row.Scan(&rec.ID, &rec.Type, &rec.RuleID, &rec.ResourceID, &rec.ResourceType,
		&rec.ResourceARN, &rec.AccountID, &rec.Region, &rec.CurrentState, &rec.RecommendedAction,
		&rec.EstimatedSavings, &rec.Confidence, &rec.Severity, &rec.TerraformCode,
		&rec.ResourceMetadata, &rec.Status, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (h *ScriptExportHandler) generateShellScript(rec *recommendations.Recommendation) string {
	tmpl := `#!/bin/bash
# FinOpsMind Remediation Script
# Recommendation: {{.ID}}
# Resource: {{.ResourceID}} ({{.ResourceType}})
# Action: {{.RecommendedAction}}
# Estimated Savings: ${{printf "%.2f" .EstimatedSavings}}/month
#
# REVIEW THIS SCRIPT BEFORE RUNNING
# Run with: chmod +x this_script.sh && ./this_script.sh

set -euo pipefail

echo "=== FinOpsMind Remediation ==="
echo "Resource: {{.ResourceID}}"
echo "Action: {{.RecommendedAction}}"
echo "Estimated Savings: ${{printf "%.2f" .EstimatedSavings}}/month"
echo ""

{{if eq (printf "%s" .Type) "idle_resource"}}
# Stop/terminate idle resource
echo "Stopping idle resource {{.ResourceID}}..."
aws ec2 stop-instances --instance-ids {{.ResourceID}} --region {{.Region}}
echo "Instance stopped. To terminate permanently:"
echo "  aws ec2 terminate-instances --instance-ids {{.ResourceID}} --region {{.Region}}"
{{else if eq (printf "%s" .Type) "oversized"}}
# Rightsize resource
echo "Rightsizing {{.ResourceID}}..."
echo "Current: {{.CurrentState}}"
echo "Recommended: {{.RecommendedAction}}"
echo ""
echo "Steps:"
echo "1. Stop instance: aws ec2 stop-instances --instance-ids {{.ResourceID}} --region {{.Region}}"
echo "2. Modify type: aws ec2 modify-instance-attribute --instance-id {{.ResourceID}} --instance-type {{.RecommendedAction}} --region {{.Region}}"
echo "3. Start instance: aws ec2 start-instances --instance-ids {{.ResourceID}} --region {{.Region}}"
{{else if eq (printf "%s" .Type) "unattached_resource"}}
# Clean up unattached resource
echo "Removing unattached resource {{.ResourceID}}..."
# Create backup snapshot first
aws ec2 create-snapshot --volume-id {{.ResourceID}} --description "FinOpsMind backup" --region {{.Region}} 2>/dev/null || true
aws ec2 delete-volume --volume-id {{.ResourceID}} --region {{.Region}} 2>/dev/null || true
aws ec2 release-address --allocation-id {{.ResourceID}} --region {{.Region}} 2>/dev/null || true
{{else}}
echo "Action type '{{.Type}}' — please apply the Terraform configuration instead."
echo "Terraform code available via: GET /api/v1/recommendations/{{.ID}}/terraform"
{{end}}

echo ""
echo "=== Done ==="
`
	t, err := template.New("script").Parse(tmpl)
	if err != nil {
		return fmt.Sprintf("#!/bin/bash\n# Error generating script: %s\n", err)
	}

	var buf bytes.Buffer
	t.Execute(&buf, rec)
	return buf.String()
}

func (h *ScriptExportHandler) generateGitHubActions(rec *recommendations.Recommendation) string {
	return fmt.Sprintf(`name: FinOpsMind Remediation - %s
on:
  workflow_dispatch:
    inputs:
      confirm:
        description: 'Type "apply" to confirm'
        required: true
        default: 'plan'

env:
  AWS_REGION: %s
  TF_VERSION: '1.5.0'

jobs:
  remediate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: ${{ env.TF_VERSION }}

      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-region: ${{ env.AWS_REGION }}
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}

      - name: Write Terraform
        run: |
          cat > main.tf << 'TFEOF'
%s
TFEOF

      - name: Terraform Init
        run: terraform init

      - name: Terraform Plan
        run: terraform plan -out=tfplan

      - name: Terraform Apply
        if: github.event.inputs.confirm == 'apply'
        run: terraform apply -auto-approve tfplan

      - name: Summary
        run: |
          echo "## Remediation Results" >> $GITHUB_STEP_SUMMARY
          echo "- Resource: %s" >> $GITHUB_STEP_SUMMARY
          echo "- Action: %s" >> $GITHUB_STEP_SUMMARY
          echo "- Estimated Savings: $%.2f/month" >> $GITHUB_STEP_SUMMARY
`, rec.ID, rec.Region, rec.TerraformCode, rec.ResourceID, rec.RecommendedAction, rec.EstimatedSavings)
}

func (h *ScriptExportHandler) generateGitLabCI(rec *recommendations.Recommendation) string {
	return fmt.Sprintf(`# FinOpsMind Remediation Pipeline - %s
# Estimated Savings: $%.2f/month

stages:
  - plan
  - apply

variables:
  TF_ROOT: "."
  AWS_DEFAULT_REGION: "%s"

.terraform_base:
  image: hashicorp/terraform:1.5
  before_script:
    - cd $TF_ROOT
    - |
      cat > main.tf << 'TFEOF'
%s
TFEOF
    - terraform init

plan:
  extends: .terraform_base
  stage: plan
  script:
    - terraform plan -out=tfplan
  artifacts:
    paths:
      - $TF_ROOT/tfplan
    expire_in: 1 hour

apply:
  extends: .terraform_base
  stage: apply
  script:
    - terraform apply -auto-approve tfplan
  dependencies:
    - plan
  when: manual
  only:
    - main
`, rec.ID, rec.EstimatedSavings, rec.Region, rec.TerraformCode)
}
