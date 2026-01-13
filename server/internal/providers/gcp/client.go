package gcp

import (
	"context"
	"fmt"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
)

// Client wraps the GCP Compute client for migration operations
type Client struct {
	instancesClient *compute.InstancesClient
	imagesClient    *compute.ImagesClient
	disksClient     *compute.DisksClient
	ctx             context.Context
	projectID       string
	zone            string
}

// Config holds GCP configuration
type Config struct {
	ProjectID       string
	Zone            string
	CredentialsFile string
}

// NewClient creates a new GCP client
func NewClient(cfg Config) (*Client, error) {
	ctx := context.Background()

	opts := []option.ClientOption{}
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}

	instancesClient, err := compute.NewInstancesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create instances client: %w", err)
	}

	imagesClient, err := compute.NewImagesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create images client: %w", err)
	}

	disksClient, err := compute.NewDisksRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	return &Client{
		instancesClient: instancesClient,
		imagesClient:    imagesClient,
		disksClient:     disksClient,
		ctx:             ctx,
		projectID:       cfg.ProjectID,
		zone:            cfg.Zone,
	}, nil
}

// Close closes all clients
func (c *Client) Close() {
	c.instancesClient.Close()
	c.imagesClient.Close()
	c.disksClient.Close()
}

// CreateImageFromGCS creates a GCE image from a file in GCS
func (c *Client) CreateImageFromGCS(imageName, gcsURI, description string) error {
	req := &computepb.InsertImageRequest{
		Project: c.projectID,
		ImageResource: &computepb.Image{
			Name:        proto.String(imageName),
			Description: proto.String(description),
			RawDisk: &computepb.RawDisk{
				Source: proto.String(gcsURI),
			},
		},
	}

	op, err := c.imagesClient.Insert(c.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	// Wait for operation to complete
	return c.waitForOperation(op, "image creation")
}

// CreateInstanceFromImage creates a GCE instance from an image
func (c *Client) CreateInstanceFromImage(instanceName, imageName, machineType, network, subnet string) error {
	imageURL := fmt.Sprintf("projects/%s/global/images/%s", c.projectID, imageName)
	machineTypeURL := fmt.Sprintf("zones/%s/machineTypes/%s", c.zone, machineType)
	networkURL := fmt.Sprintf("projects/%s/global/networks/%s", c.projectID, network)
	subnetURL := fmt.Sprintf("projects/%s/regions/%s/subnetworks/%s", c.projectID, getRegionFromZone(c.zone), subnet)

	req := &computepb.InsertInstanceRequest{
		Project: c.projectID,
		Zone:    c.zone,
		InstanceResource: &computepb.Instance{
			Name:        proto.String(instanceName),
			MachineType: proto.String(machineTypeURL),
			Disks: []*computepb.AttachedDisk{
				{
					AutoDelete: proto.Bool(true),
					Boot:       proto.Bool(true),
					Type:       proto.String("PERSISTENT"),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						SourceImage: proto.String(imageURL),
						DiskSizeGb:  proto.Int64(100), // Will be overridden by image size
					},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Network:    proto.String(networkURL),
					Subnetwork: proto.String(subnetURL),
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name: proto.String("External NAT"),
							Type: proto.String("ONE_TO_ONE_NAT"),
						},
					},
				},
			},
		},
	}

	op, err := c.instancesClient.Insert(c.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	return c.waitForOperation(op, "instance creation")
}

// GetInstanceInfo returns details about a GCE instance
func (c *Client) GetInstanceInfo(instanceName string) (map[string]interface{}, error) {
	req := &computepb.GetInstanceRequest{
		Project:  c.projectID,
		Zone:     c.zone,
		Instance: instanceName,
	}

	instance, err := c.instancesClient.Get(c.ctx, req)
	if err != nil {
		return nil, err
	}

	info := map[string]interface{}{
		"name":         instance.GetName(),
		"status":       instance.GetStatus(),
		"machine_type": instance.GetMachineType(),
		"zone":         instance.GetZone(),
	}

	// Get network info
	if len(instance.NetworkInterfaces) > 0 {
		ni := instance.NetworkInterfaces[0]
		info["internal_ip"] = ni.GetNetworkIP()
		if len(ni.AccessConfigs) > 0 {
			info["external_ip"] = ni.AccessConfigs[0].GetNatIP()
		}
	}

	return info, nil
}

// StartInstance starts a stopped GCE instance
func (c *Client) StartInstance(instanceName string) error {
	req := &computepb.StartInstanceRequest{
		Project:  c.projectID,
		Zone:     c.zone,
		Instance: instanceName,
	}

	op, err := c.instancesClient.Start(c.ctx, req)
	if err != nil {
		return err
	}

	return c.waitForOperation(op, "instance start")
}

// StopInstance stops a running GCE instance
func (c *Client) StopInstance(instanceName string) error {
	req := &computepb.StopInstanceRequest{
		Project:  c.projectID,
		Zone:     c.zone,
		Instance: instanceName,
	}

	op, err := c.instancesClient.Stop(c.ctx, req)
	if err != nil {
		return err
	}

	return c.waitForOperation(op, "instance stop")
}

// CreateSnapshot creates a snapshot of a disk
func (c *Client) CreateSnapshot(diskName, snapshotName string) error {
	req := &computepb.CreateSnapshotDiskRequest{
		Project: c.projectID,
		Zone:    c.zone,
		Disk:    diskName,
		SnapshotResource: &computepb.Snapshot{
			Name: proto.String(snapshotName),
		},
	}

	op, err := c.disksClient.CreateSnapshot(c.ctx, req)
	if err != nil {
		return err
	}

	return c.waitForOperation(op, "snapshot creation")
}

// EstimateMachineType suggests an appropriate GCE machine type based on VM specs
func EstimateMachineType(cpuCount int, memoryGB float64) string {
	// GCE has predefined and custom machine types
	// Using n2 series for general purpose
	if cpuCount <= 2 && memoryGB <= 8 {
		return "n2-standard-2"
	} else if cpuCount <= 4 && memoryGB <= 16 {
		return "n2-standard-4"
	} else if cpuCount <= 8 && memoryGB <= 32 {
		return "n2-standard-8"
	} else if cpuCount <= 16 && memoryGB <= 64 {
		return "n2-standard-16"
	} else if cpuCount <= 32 && memoryGB <= 128 {
		return "n2-standard-32"
	} else if cpuCount <= 48 && memoryGB <= 192 {
		return "n2-standard-48"
	} else if cpuCount <= 64 && memoryGB <= 256 {
		return "n2-standard-64"
	} else {
		return "n2-standard-80"
	}
}

// waitForOperation waits for a GCE operation to complete
func (c *Client) waitForOperation(op *compute.Operation, operationName string) error {
	for {
		if op.Done() {
			if op.Proto().GetError() != nil {
				return fmt.Errorf("%s failed: %s", operationName, op.Proto().GetError().GetErrors()[0].GetMessage())
			}
			return nil
		}
		time.Sleep(5 * time.Second)
	}
}

func getRegionFromZone(zone string) string {
	// Remove the last part (e.g., us-central1-a -> us-central1)
	if len(zone) > 2 {
		return zone[:len(zone)-2]
	}
	return zone
}
