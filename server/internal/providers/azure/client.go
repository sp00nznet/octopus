package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// Client wraps the Azure Compute client for migration operations
type Client struct {
	vmClient       *armcompute.VirtualMachinesClient
	disksClient    *armcompute.DisksClient
	imagesClient   *armcompute.ImagesClient
	ctx            context.Context
	subscriptionID string
	resourceGroup  string
	location       string
}

// Config holds Azure configuration
type Config struct {
	SubscriptionID string
	ResourceGroup  string
	TenantID       string
	ClientID       string
	ClientSecret   string
	Location       string
}

// NewClient creates a new Azure client
func NewClient(cfg Config) (*Client, error) {
	ctx := context.Background()

	cred, err := azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM client: %w", err)
	}

	disksClient, err := armcompute.NewDisksClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	imagesClient, err := armcompute.NewImagesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create images client: %w", err)
	}

	return &Client{
		vmClient:       vmClient,
		disksClient:    disksClient,
		imagesClient:   imagesClient,
		ctx:            ctx,
		subscriptionID: cfg.SubscriptionID,
		resourceGroup:  cfg.ResourceGroup,
		location:       cfg.Location,
	}, nil
}

// CreateImageFromVHD creates an Azure managed image from a VHD in blob storage
func (c *Client) CreateImageFromVHD(imageName, vhdURI, osType string) error {
	var osTypeEnum armcompute.OperatingSystemTypes
	if osType == "windows" {
		osTypeEnum = armcompute.OperatingSystemTypesWindows
	} else {
		osTypeEnum = armcompute.OperatingSystemTypesLinux
	}

	image := armcompute.Image{
		Location: to.Ptr(c.location),
		Properties: &armcompute.ImageProperties{
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:  to.Ptr(osTypeEnum),
					BlobURI: to.Ptr(vhdURI),
					OSState: to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
				},
			},
		},
	}

	poller, err := c.imagesClient.BeginCreateOrUpdate(c.ctx, c.resourceGroup, imageName, image, nil)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	_, err = poller.PollUntilDone(c.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed waiting for image creation: %w", err)
	}

	return nil
}

// CreateVMFromImage creates an Azure VM from a managed image
func (c *Client) CreateVMFromImage(vmName, imageName, vmSize, vnetName, subnetName, adminUsername, adminPassword string) error {
	imageID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s",
		c.subscriptionID, c.resourceGroup, imageName)

	subnetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
		c.subscriptionID, c.resourceGroup, vnetName, subnetName)

	// Create NIC first
	nicName := vmName + "-nic"
	nicID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkInterfaces/%s",
		c.subscriptionID, c.resourceGroup, nicName)

	vm := armcompute.VirtualMachine{
		Location: to.Ptr(c.location),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(vmSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					ID: to.Ptr(imageID),
				},
				OSDisk: &armcompute.OSDisk{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(vmName),
				AdminUsername: to.Ptr(adminUsername),
				AdminPassword: to.Ptr(adminPassword),
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(nicID),
					},
				},
			},
		},
	}

	// Note: In a real implementation, you'd create the NIC first
	_ = subnetID // Would be used for NIC creation

	poller, err := c.vmClient.BeginCreateOrUpdate(c.ctx, c.resourceGroup, vmName, vm, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	_, err = poller.PollUntilDone(c.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed waiting for VM creation: %w", err)
	}

	return nil
}

// GetVMInfo returns details about an Azure VM
func (c *Client) GetVMInfo(vmName string) (map[string]interface{}, error) {
	vm, err := c.vmClient.Get(c.ctx, c.resourceGroup, vmName, nil)
	if err != nil {
		return nil, err
	}

	info := map[string]interface{}{
		"name":     *vm.Name,
		"location": *vm.Location,
		"vm_size":  string(*vm.Properties.HardwareProfile.VMSize),
		"vm_id":    *vm.Properties.VMID,
	}

	if vm.Properties.ProvisioningState != nil {
		info["provisioning_state"] = *vm.Properties.ProvisioningState
	}

	return info, nil
}

// StartVM starts a stopped Azure VM
func (c *Client) StartVM(vmName string) error {
	poller, err := c.vmClient.BeginStart(c.ctx, c.resourceGroup, vmName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(c.ctx, nil)
	return err
}

// StopVM deallocates an Azure VM
func (c *Client) StopVM(vmName string) error {
	poller, err := c.vmClient.BeginDeallocate(c.ctx, c.resourceGroup, vmName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(c.ctx, nil)
	return err
}

// CreateSnapshot creates a snapshot of a managed disk
func (c *Client) CreateSnapshot(diskName, snapshotName string) error {
	// Get disk info
	disk, err := c.disksClient.Get(c.ctx, c.resourceGroup, diskName, nil)
	if err != nil {
		return fmt.Errorf("failed to get disk: %w", err)
	}

	snapshot := armcompute.Snapshot{
		Location: to.Ptr(c.location),
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: disk.ID,
			},
		},
	}

	snapshotsClient, err := armcompute.NewSnapshotsClient(c.subscriptionID, nil, nil)
	if err != nil {
		return err
	}

	poller, err := snapshotsClient.BeginCreateOrUpdate(c.ctx, c.resourceGroup, snapshotName, snapshot, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(c.ctx, nil)
	return err
}

// EstimateVMSize suggests an appropriate Azure VM size based on VM specs
func EstimateVMSize(cpuCount int, memoryGB float64) string {
	// Using D-series for general purpose
	if cpuCount <= 2 && memoryGB <= 8 {
		return "Standard_D2s_v3"
	} else if cpuCount <= 4 && memoryGB <= 16 {
		return "Standard_D4s_v3"
	} else if cpuCount <= 8 && memoryGB <= 32 {
		return "Standard_D8s_v3"
	} else if cpuCount <= 16 && memoryGB <= 64 {
		return "Standard_D16s_v3"
	} else if cpuCount <= 32 && memoryGB <= 128 {
		return "Standard_D32s_v3"
	} else if cpuCount <= 48 && memoryGB <= 192 {
		return "Standard_D48s_v3"
	} else if cpuCount <= 64 && memoryGB <= 256 {
		return "Standard_D64s_v3"
	} else {
		return "Standard_E64s_v3" // High memory option
	}
}
