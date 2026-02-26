package remediation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/finopsmind/backend/internal/model"
)

// AWSExecutor performs real AWS API remediation actions.
type AWSExecutor struct {
	logger *slog.Logger
}

// NewAWSExecutor creates a new AWSExecutor.
func NewAWSExecutor(logger *slog.Logger) *AWSExecutor {
	return &AWSExecutor{logger: logger}
}

func (e *AWSExecutor) newEC2Client(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) (*ec2.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(action.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return ec2.NewFromConfig(cfg), nil
}

func (e *AWSExecutor) newS3Client(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(action.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return s3.NewFromConfig(cfg), nil
}

// Execute dispatches to the appropriate AWS API call.
func (e *AWSExecutor) Execute(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	switch action.Type {
	case model.RemediationTypeResizeInstance:
		return e.resizeInstance(ctx, action, creds)
	case model.RemediationTypeStopInstance:
		return e.stopInstance(ctx, action, creds)
	case model.RemediationTypeTerminateInstance:
		return e.terminateInstance(ctx, action, creds)
	case model.RemediationTypeDeleteVolume:
		return e.deleteVolume(ctx, action, creds)
	case model.RemediationTypeUpgradeStorage:
		return e.upgradeStorage(ctx, action, creds)
	case model.RemediationTypeReleaseEIP:
		return e.releaseEIP(ctx, action, creds)
	case model.RemediationTypeDeleteSnapshot:
		return e.deleteSnapshot(ctx, action, creds)
	case model.RemediationTypeApplyLifecyclePolicy:
		return e.applyLifecyclePolicy(ctx, action, creds)
	default:
		return fmt.Errorf("unsupported AWS remediation type: %s", action.Type)
	}
}

// Rollback dispatches to the appropriate rollback action.
func (e *AWSExecutor) Rollback(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	switch action.Type {
	case model.RemediationTypeResizeInstance:
		return e.rollbackResize(ctx, action, creds)
	case model.RemediationTypeStopInstance:
		return e.startInstance(ctx, action, creds)
	default:
		return fmt.Errorf("rollback not supported for type: %s", action.Type)
	}
}

func (e *AWSExecutor) resizeInstance(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	instanceID := action.ResourceID
	desiredType := ""
	if action.DesiredState != nil {
		if t, ok := action.DesiredState["instance_type"].(string); ok {
			desiredType = t
		}
	}
	if desiredType == "" {
		return fmt.Errorf("desired_state.instance_type is required for resize")
	}

	e.logger.Info("stopping instance for resize", "instance_id", instanceID)

	// Stop instance
	_, err = client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	// Wait for stopped
	waiter := ec2.NewInstanceStoppedWaiter(client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, WaitTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for instance to stop: %w", err)
	}

	// Modify instance type
	_, err = client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
		InstanceId: aws.String(instanceID),
		InstanceType: &ec2types.AttributeValue{
			Value: aws.String(desiredType),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to modify instance type: %w", err)
	}

	// Start instance
	_, err = client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to start instance after resize: %w", err)
	}

	e.logger.Info("instance resized successfully", "instance_id", instanceID, "new_type", desiredType)
	return nil
}

func (e *AWSExecutor) rollbackResize(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	if action.RollbackData == nil {
		return fmt.Errorf("no rollback data")
	}
	originalType, ok := action.RollbackData["original_instance_type"].(string)
	if !ok {
		return fmt.Errorf("rollback data missing original_instance_type")
	}

	// Create a temporary action with the original type as desired
	rollbackAction := *action
	rollbackAction.DesiredState = map[string]any{"instance_type": originalType}
	return e.resizeInstance(ctx, &rollbackAction, creds)
}

func (e *AWSExecutor) stopInstance(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	_, err = client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{action.ResourceID},
	})
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	e.logger.Info("instance stopped", "instance_id", action.ResourceID)
	return nil
}

func (e *AWSExecutor) startInstance(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	_, err = client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{action.ResourceID},
	})
	if err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	e.logger.Info("instance started (rollback)", "instance_id", action.ResourceID)
	return nil
}

func (e *AWSExecutor) terminateInstance(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{action.ResourceID},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instance: %w", err)
	}

	e.logger.Info("instance terminated", "instance_id", action.ResourceID)
	return nil
}

func (e *AWSExecutor) deleteVolume(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	// Create snapshot before deleting (for rollback)
	snapOut, err := client.CreateSnapshot(ctx, &ec2.CreateSnapshotInput{
		VolumeId:    aws.String(action.ResourceID),
		Description: aws.String(fmt.Sprintf("FinOpsMind backup before delete - action %s", action.ID)),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSnapshot,
				Tags: []ec2types.Tag{
					{Key: aws.String("finopsmind:action"), Value: aws.String(action.ID.String())},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create backup snapshot: %w", err)
	}

	e.logger.Info("backup snapshot created", "snapshot_id", *snapOut.SnapshotId, "volume_id", action.ResourceID)

	// Store snapshot ID in rollback data
	if action.RollbackData == nil {
		action.RollbackData = make(map[string]any)
	}
	action.RollbackData["backup_snapshot_id"] = *snapOut.SnapshotId

	// Delete volume
	_, err = client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
		VolumeId: aws.String(action.ResourceID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	e.logger.Info("volume deleted", "volume_id", action.ResourceID)
	return nil
}

func (e *AWSExecutor) upgradeStorage(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	targetType := ec2types.VolumeTypeGp3
	if action.DesiredState != nil {
		if t, ok := action.DesiredState["volume_type"].(string); ok {
			targetType = ec2types.VolumeType(t)
		}
	}

	_, err = client.ModifyVolume(ctx, &ec2.ModifyVolumeInput{
		VolumeId:   aws.String(action.ResourceID),
		VolumeType: targetType,
	})
	if err != nil {
		return fmt.Errorf("failed to modify volume: %w", err)
	}

	e.logger.Info("volume upgraded", "volume_id", action.ResourceID, "new_type", targetType)
	return nil
}

func (e *AWSExecutor) releaseEIP(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	// ResourceID is the allocation ID for EIPs
	_, err = client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
		AllocationId: aws.String(action.ResourceID),
	})
	if err != nil {
		return fmt.Errorf("failed to release EIP: %w", err)
	}

	e.logger.Info("EIP released", "allocation_id", action.ResourceID)
	return nil
}

func (e *AWSExecutor) deleteSnapshot(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	client, err := e.newEC2Client(ctx, action, creds)
	if err != nil {
		return err
	}

	_, err = client.DeleteSnapshot(ctx, &ec2.DeleteSnapshotInput{
		SnapshotId: aws.String(action.ResourceID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	e.logger.Info("snapshot deleted", "snapshot_id", action.ResourceID)
	return nil
}

func (e *AWSExecutor) applyLifecyclePolicy(ctx context.Context, action *model.RemediationAction, creds *AWSCreds) error {
	s3Client, err := e.newS3Client(ctx, action, creds)
	if err != nil {
		return err
	}

	bucketName := action.ResourceID
	transitionDays := int32(30)
	expirationDays := int32(365)

	if action.DesiredState != nil {
		if d, ok := action.DesiredState["transition_days"].(float64); ok {
			transitionDays = int32(d)
		}
		if d, ok := action.DesiredState["expiration_days"].(float64); ok {
			expirationDays = int32(d)
		}
	}

	_, err = s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &s3types.BucketLifecycleConfiguration{
			Rules: []s3types.LifecycleRule{
				{
					ID:     aws.String("finopsmind-lifecycle"),
					Status: s3types.ExpirationStatusEnabled,
					Filter: &s3types.LifecycleRuleFilter{
					Prefix: aws.String(""),
				},
					Transitions: []s3types.Transition{
						{
							Days:         aws.Int32(transitionDays),
							StorageClass: s3types.TransitionStorageClassIntelligentTiering,
						},
					},
					Expiration: &s3types.LifecycleExpiration{
						Days: aws.Int32(expirationDays),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to apply lifecycle policy: %w", err)
	}

	e.logger.Info("lifecycle policy applied", "bucket", bucketName)
	return nil
}
