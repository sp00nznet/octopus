package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Client wraps the AWS EC2 client for migration operations
type Client struct {
	ec2Client *ec2.Client
	ctx       context.Context
	region    string
}

// Config holds AWS configuration
type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// NewClient creates a new AWS client
func NewClient(cfg Config) (*Client, error) {
	ctx := context.Background()

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		ec2Client: ec2.NewFromConfig(awsCfg),
		ctx:       ctx,
		region:    cfg.Region,
	}, nil
}

// ImportVMImage imports a VM image from S3 to create an AMI
func (c *Client) ImportVMImage(s3Bucket, s3Key, description string, diskFormat string) (string, error) {
	input := &ec2.ImportImageInput{
		Description: aws.String(description),
		DiskContainers: []types.ImageDiskContainer{
			{
				Description: aws.String(description),
				Format:      aws.String(diskFormat),
				UserBucket: &types.UserBucket{
					S3Bucket: aws.String(s3Bucket),
					S3Key:    aws.String(s3Key),
				},
			},
		},
	}

	result, err := c.ec2Client.ImportImage(c.ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to import image: %w", err)
	}

	return *result.ImportTaskId, nil
}

// GetImportStatus returns the status of an import task
func (c *Client) GetImportStatus(taskID string) (string, int, error) {
	input := &ec2.DescribeImportImageTasksInput{
		ImportTaskIds: []string{taskID},
	}

	result, err := c.ec2Client.DescribeImportImageTasks(c.ctx, input)
	if err != nil {
		return "", 0, err
	}

	if len(result.ImportImageTasks) == 0 {
		return "", 0, fmt.Errorf("import task not found")
	}

	task := result.ImportImageTasks[0]
	progress := 0
	if task.Progress != nil {
		fmt.Sscanf(*task.Progress, "%d", &progress)
	}

	return *task.Status, progress, nil
}

// CreateInstanceFromAMI launches an EC2 instance from an AMI
func (c *Client) CreateInstanceFromAMI(amiID, instanceType, subnetID, securityGroupID string, preserveMAC bool) (string, error) {
	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		InstanceType: types.InstanceType(instanceType),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		SubnetId:     aws.String(subnetID),
		SecurityGroupIds: []string{
			securityGroupID,
		},
	}

	// Note: AWS doesn't support preserving MAC addresses directly
	// The MAC is assigned by AWS VPC

	result, err := c.ec2Client.RunInstances(c.ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to launch instance: %w", err)
	}

	if len(result.Instances) == 0 {
		return "", fmt.Errorf("no instance created")
	}

	instanceID := *result.Instances[0].InstanceId

	// Wait for instance to be running
	waiter := ec2.NewInstanceRunningWaiter(c.ec2Client)
	err = waiter.Wait(c.ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 10*time.Minute)
	if err != nil {
		return instanceID, fmt.Errorf("instance created but failed to start: %w", err)
	}

	return instanceID, nil
}

// GetInstanceInfo returns details about an EC2 instance
func (c *Client) GetInstanceInfo(instanceID string) (map[string]interface{}, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := c.ec2Client.DescribeInstances(c.ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance not found")
	}

	instance := result.Reservations[0].Instances[0]

	info := map[string]interface{}{
		"instance_id":    *instance.InstanceId,
		"instance_type":  string(instance.InstanceType),
		"state":          string(instance.State.Name),
		"private_ip":     safeString(instance.PrivateIpAddress),
		"public_ip":      safeString(instance.PublicIpAddress),
		"vpc_id":         safeString(instance.VpcId),
		"subnet_id":      safeString(instance.SubnetId),
		"launch_time":    instance.LaunchTime,
	}

	return info, nil
}

// CreateSnapshot creates an EBS snapshot
func (c *Client) CreateSnapshot(volumeID, description string) (string, error) {
	input := &ec2.CreateSnapshotInput{
		VolumeId:    aws.String(volumeID),
		Description: aws.String(description),
	}

	result, err := c.ec2Client.CreateSnapshot(c.ctx, input)
	if err != nil {
		return "", err
	}

	return *result.SnapshotId, nil
}

// WaitForSnapshot waits for a snapshot to complete
func (c *Client) WaitForSnapshot(snapshotID string) error {
	waiter := ec2.NewSnapshotCompletedWaiter(c.ec2Client)
	return waiter.Wait(c.ctx, &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
	}, 30*time.Minute)
}

// EstimateInstanceType suggests an appropriate EC2 instance type based on VM specs
func EstimateInstanceType(cpuCount int, memoryGB float64) string {
	// Simple mapping - in production this would be more sophisticated
	if cpuCount <= 2 && memoryGB <= 4 {
		return "t3.medium"
	} else if cpuCount <= 4 && memoryGB <= 16 {
		return "m5.xlarge"
	} else if cpuCount <= 8 && memoryGB <= 32 {
		return "m5.2xlarge"
	} else if cpuCount <= 16 && memoryGB <= 64 {
		return "m5.4xlarge"
	} else if cpuCount <= 32 && memoryGB <= 128 {
		return "m5.8xlarge"
	} else if cpuCount <= 48 && memoryGB <= 192 {
		return "m5.12xlarge"
	} else if cpuCount <= 64 && memoryGB <= 256 {
		return "m5.16xlarge"
	} else {
		return "m5.24xlarge"
	}
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
