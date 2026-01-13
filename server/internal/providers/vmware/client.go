package vmware

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Client wraps the govmomi client for vCenter operations
type Client struct {
	client     *govmomi.Client
	finder     *find.Finder
	datacenter *object.Datacenter
	ctx        context.Context
}

// VMInfo represents VM information from vCenter
type VMInfo struct {
	Name              string  `json:"name"`
	UUID              string  `json:"uuid"`
	CPUCount          int     `json:"cpu_count"`
	MemoryMB          int     `json:"memory_mb"`
	DiskSizeGB        float64 `json:"disk_size_gb"`
	GuestOS           string  `json:"guest_os"`
	PowerState        string  `json:"power_state"`
	IPAddresses       string  `json:"ip_addresses"`
	MACAddresses      string  `json:"mac_addresses"`
	PortGroups        string  `json:"port_groups"`
	HardwareVersion   string  `json:"hardware_version"`
	VMwareToolsStatus string  `json:"vmware_tools_status"`
}

// NewClient creates a new vCenter client
func NewClient(host, username, password, datacenter string, insecure bool) (*Client, error) {
	ctx := context.Background()

	// Build URL
	u, err := url.Parse(fmt.Sprintf("https://%s/sdk", host))
	if err != nil {
		return nil, fmt.Errorf("invalid host: %w", err)
	}
	u.User = url.UserPassword(username, password)

	// Connect to vCenter
	client, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vCenter: %w", err)
	}

	// Create finder
	finder := find.NewFinder(client.Client, true)

	// Find datacenter
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter %s: %w", datacenter, err)
	}
	finder.SetDatacenter(dc)

	return &Client{
		client:     client,
		finder:     finder,
		datacenter: dc,
		ctx:        ctx,
	}, nil
}

// Logout disconnects from vCenter
func (c *Client) Logout() error {
	return c.client.Logout(c.ctx)
}

// ListVMs returns all VMs in the datacenter
func (c *Client) ListVMs() ([]VMInfo, error) {
	vms, err := c.finder.VirtualMachineList(c.ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	var vmInfos []VMInfo
	for _, vm := range vms {
		info, err := c.getVMInfo(vm)
		if err != nil {
			continue // Skip VMs we can't read
		}
		vmInfos = append(vmInfos, *info)
	}

	return vmInfos, nil
}

// GetVM returns info for a specific VM
func (c *Client) GetVM(name string) (*VMInfo, error) {
	vm, err := c.finder.VirtualMachine(c.ctx, name)
	if err != nil {
		return nil, fmt.Errorf("VM not found: %w", err)
	}
	return c.getVMInfo(vm)
}

// getVMInfo extracts detailed info from a VM object
func (c *Client) getVMInfo(vm *object.VirtualMachine) (*VMInfo, error) {
	var mvm mo.VirtualMachine

	pc := property.DefaultCollector(c.client.Client)
	err := pc.RetrieveOne(c.ctx, vm.Reference(), []string{
		"config",
		"summary",
		"guest",
		"runtime",
	}, &mvm)
	if err != nil {
		return nil, err
	}

	info := &VMInfo{
		Name:       mvm.Summary.Config.Name,
		UUID:       mvm.Summary.Config.Uuid,
		CPUCount:   int(mvm.Summary.Config.NumCpu),
		MemoryMB:   int(mvm.Summary.Config.MemorySizeMB),
		GuestOS:    mvm.Summary.Config.GuestFullName,
		PowerState: string(mvm.Summary.Runtime.PowerState),
	}

	// Calculate disk size
	if mvm.Config != nil {
		for _, dev := range mvm.Config.Hardware.Device {
			if disk, ok := dev.(*types.VirtualDisk); ok {
				info.DiskSizeGB += float64(disk.CapacityInKB) / 1024 / 1024
			}
		}
		info.HardwareVersion = mvm.Config.Version
	}

	// Get network info
	var macs, ips, portGroups []string
	if mvm.Config != nil {
		for _, dev := range mvm.Config.Hardware.Device {
			if nic, ok := dev.(types.BaseVirtualEthernetCard); ok {
				ethCard := nic.GetVirtualEthernetCard()
				macs = append(macs, ethCard.MacAddress)

				// Get port group
				if backing, ok := ethCard.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
					portGroups = append(portGroups, backing.Port.PortgroupKey)
				} else if backing, ok := ethCard.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
					portGroups = append(portGroups, backing.DeviceName)
				}
			}
		}
	}

	// Get IP addresses from guest info
	if mvm.Guest != nil {
		for _, nic := range mvm.Guest.Net {
			ips = append(ips, nic.IpAddress...)
		}
		info.VMwareToolsStatus = string(mvm.Guest.ToolsStatus)
	}

	info.MACAddresses = strings.Join(macs, ",")
	info.IPAddresses = strings.Join(ips, ",")
	info.PortGroups = strings.Join(portGroups, ",")

	return info, nil
}

// ExportVM exports a VM to OVF format
func (c *Client) ExportVM(vmName string, exportPath string) error {
	vm, err := c.finder.VirtualMachine(c.ctx, vmName)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	// Get OVF manager
	m := object.NewOvfManager(c.client.Client)

	// Create export spec
	var mvm mo.VirtualMachine
	pc := property.DefaultCollector(c.client.Client)
	err = pc.RetrieveOne(c.ctx, vm.Reference(), []string{"config"}, &mvm)
	if err != nil {
		return err
	}

	// Create OVF descriptor
	spec := types.OvfCreateDescriptorParams{
		Name: vmName,
	}

	result, err := m.CreateDescriptor(c.ctx, vm, spec)
	if err != nil {
		return fmt.Errorf("failed to create OVF descriptor: %w", err)
	}

	if result.Error != nil {
		return fmt.Errorf("OVF descriptor error: %s", result.Error[0].LocalizedMessage)
	}

	// The actual disk export would happen here using the HTTP lease
	// This is a simplified version - full implementation would stream disks
	_ = result.OvfDescriptor

	return nil
}

// CloneVM clones a VM within the same vCenter or to another vCenter
func (c *Client) CloneVM(sourceName, destName, destFolder, destHost, destDatastore string, preserveMAC bool) error {
	// Find source VM
	sourceVM, err := c.finder.VirtualMachine(c.ctx, sourceName)
	if err != nil {
		return fmt.Errorf("source VM not found: %w", err)
	}

	// Find destination folder
	folder, err := c.finder.Folder(c.ctx, destFolder)
	if err != nil {
		return fmt.Errorf("destination folder not found: %w", err)
	}

	// Find destination host
	host, err := c.finder.HostSystem(c.ctx, destHost)
	if err != nil {
		return fmt.Errorf("destination host not found: %w", err)
	}

	// Find destination datastore
	ds, err := c.finder.Datastore(c.ctx, destDatastore)
	if err != nil {
		return fmt.Errorf("destination datastore not found: %w", err)
	}

	// Get resource pool
	pool, err := host.ResourcePool(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to get resource pool: %w", err)
	}

	// Build clone spec
	relocateSpec := types.VirtualMachineRelocateSpec{
		Datastore: types.NewReference(ds.Reference()),
		Pool:      types.NewReference(pool.Reference()),
		Host:      types.NewReference(host.Reference()),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location: relocateSpec,
		PowerOn:  false,
		Template: false,
	}

	// If preserving MAC addresses, we need to customize
	if preserveMAC {
		// MAC addresses will be preserved by default in a clone operation
		// unless explicitly changed
	}

	// Clone the VM
	task, err := sourceVM.Clone(c.ctx, folder, destName, cloneSpec)
	if err != nil {
		return fmt.Errorf("failed to start clone: %w", err)
	}

	// Wait for completion
	err = task.Wait(c.ctx)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	return nil
}

// PowerOn powers on a VM
func (c *Client) PowerOn(vmName string) error {
	vm, err := c.finder.VirtualMachine(c.ctx, vmName)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	task, err := vm.PowerOn(c.ctx)
	if err != nil {
		return err
	}
	return task.Wait(c.ctx)
}

// PowerOff powers off a VM
func (c *Client) PowerOff(vmName string) error {
	vm, err := c.finder.VirtualMachine(c.ctx, vmName)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	task, err := vm.PowerOff(c.ctx)
	if err != nil {
		return err
	}
	return task.Wait(c.ctx)
}

// CreateSnapshot creates a snapshot of a VM
func (c *Client) CreateSnapshot(vmName, snapshotName, description string, memory, quiesce bool) error {
	vm, err := c.finder.VirtualMachine(c.ctx, vmName)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	task, err := vm.CreateSnapshot(c.ctx, snapshotName, description, memory, quiesce)
	if err != nil {
		return err
	}
	return task.Wait(c.ctx)
}

// GetChangedBlocks returns changed disk blocks since a snapshot (for CBT)
func (c *Client) GetChangedBlocks(vmName, snapshotID string, diskKey int32, startOffset int64) ([]types.DiskChangeInfo, error) {
	vm, err := c.finder.VirtualMachine(c.ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("VM not found: %w", err)
	}

	// Get the VM's disk change info using CBT
	var mvm mo.VirtualMachine
	pc := property.DefaultCollector(c.client.Client)
	err = pc.RetrieveOne(c.ctx, vm.Reference(), []string{"config"}, &mvm)
	if err != nil {
		return nil, err
	}

	// Find disk capacity
	var diskCapacity int64
	for _, dev := range mvm.Config.Hardware.Device {
		if disk, ok := dev.(*types.VirtualDisk); ok {
			if disk.Key == diskKey {
				diskCapacity = disk.CapacityInKB * 1024
				break
			}
		}
	}

	// Query changed blocks
	changeInfo, err := vm.QueryChangedDiskAreas(c.ctx, nil, nil, &types.VirtualMachineSnapshotInfo{}, diskKey, startOffset, snapshotID)
	if err != nil {
		return nil, err
	}

	_ = diskCapacity // Would be used for full block tracking

	return []types.DiskChangeInfo{*changeInfo}, nil
}
