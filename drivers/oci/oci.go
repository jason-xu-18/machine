package oci

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/state"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/core"
	"github.com/oracle/oci-go-sdk/example/helpers"
	"github.com/oracle/oci-go-sdk/identity"
)

// Driver represents Oci Docker Machine Driver.
type Driver struct {
	*drivers.BaseDriver

	tenancy string

	CompartmentName    string
	DisplayName        string
	AvailabilityDomain string
	FaultDomain        string
	VCNName            string
	ImageName          string
	Shape              string
	SubnetName         string

	// default retry policy will retry on non-200 response
	RequestMetadata common.RequestMetadata

	InstanceID string

	DockerPort int

	resolvedIP string
}

const (
	defaultSSHUser            = "docker-user"
	defaultOciAvailableDomain = "eXkP:PHX-AD-1"
	defaultOciShape           = "VM.Standard2.1"
	defaultOciFaultDomain     = "FAULT-DOMAIN-1"
	defaultOciImageName       = "Oracle Linux 7.6"
	defaultOciVCNName         = "OCI-GOSDK-Sample-VCN"
	defaultOciSubnetName      = "OCI-GOSDK-Sample-Subnet2"
)

const (
	flagOciAvailableDomain = "oci-available-domain"
	flagOciFaultDomain     = "oci-fault-domain"
	flagOciShape           = "oci-shape"
	flagOciImageName       = "oci-image"
	flagOciVCNName         = "oci-vnet"
	flagOciSubnetName      = "oci-subnet"
)

const (
	driverName               = "oci"
	ipRange                  = "0.0.0.0/0"
	machineSecurityGroupName = "rancher-nodes"
	machineTag               = "rancher-nodes"
)

// NewDriver returns a new driver instance.
func NewDriver(hostName, storePath string) drivers.Driver {
	d := &Driver{
		BaseDriver: &drivers.BaseDriver{
			SSHUser:     defaultSSHUser,
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
	return d
}

//Stop issues a power off for the virtual machine instance.
func (d *Driver) Stop() error {
	fmt.Println("Stoping inst03")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId = &(d.InstanceID)
	request.Action = core.InstanceActionActionEnum("STOP")

	_, err = c.InstanceAction(ctx, request)
	helpers.FatalIfError(err)

	return nil
}

//Start issues a power on for the virtual machine instance.
func (d *Driver) Start() error {
	fmt.Println("Starting instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId = &(d.InstanceID)
	request.Action = core.InstanceActionActionEnum("START")

	_, err = c.InstanceAction(ctx, request)
	helpers.FatalIfError(err)

	return nil
}

// GetState returns the state of the virtual machine role instance.
func (d *Driver) GetState() (state.State, error) {
	fmt.Println("Getting state of instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.GetInstanceRequest{}
	request.InstanceId = &(d.InstanceID)

	response, err := c.GetInstance(ctx, request)
	lifecycleState := response.Instance.LifecycleState
	machineState := machineStateForLifecycleState(lifecycleState)
	log.Debugf("Determined Oci LifecycleState=%q, docker-machine state=%q",
		lifecycleState, machineState)
	return machineState, nil
}

// DriverName returns the name of the driver.
func (d *Driver) DriverName() string { return driverName }

// GetCreateFlags returns list of create flags driver accepts.
func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			Name:   flagOciAvailableDomain,
			Usage:  "The availability domain of the instance.",
			EnvVar: "OCI_AVAILABLE_DOMAIN",
			Value:  defaultOciAvailableDomain,
		},
		mcnflag.StringFlag{
			Name:   flagOciFaultDomain,
			Usage:  "A fault domain is a grouping of hardware and infrastructure within an availability domain.",
			EnvVar: "OCI_FAULT_DOMAIN",
			Value:  defaultOciFaultDomain,
		},
		mcnflag.StringFlag{
			Name:   flagOciShape,
			Usage:  "The shape of an instance. The shape determines the number of CPUs, amount of memory, and other resources allocated to the instance.",
			EnvVar: "OCI_SHAPE",
			Value:  defaultOciShape,
		},
		mcnflag.StringFlag{
			Name:   flagOciImageName,
			Usage:  "The display name of image",
			EnvVar: "OCI_IMAGE_NAME",
			Value:  defaultOciImageName,
		},
		mcnflag.StringFlag{
			Name:   flagOciVCNName,
			Usage:  "The display name of VCN",
			EnvVar: "OCI_VCN_NAME",
			Value:  defaultOciVCNName,
		},
		mcnflag.StringFlag{
			Name:   flagOciSubnetName,
			Usage:  "The display name of subnet",
			EnvVar: "OCI_SUBNET_NAME",
			Value:  defaultOciSubnetName,
		},
	}
}

// SetConfigFromFlags initializes driver values from the command line values
// and checks if the arguments have values.
func (d *Driver) SetConfigFromFlags(fl drivers.DriverOptions) error {
	// Initialize driver context for machine

	// Required string flags
	flags := []struct {
		target *string
		flag   string
	}{
		{&d.ImageName, flagOciImageName},
		{&d.VCNName, flagOciVCNName},
		{&d.SubnetName, flagOciSubnetName},
		{&d.AvailabilityDomain, flagOciAvailableDomain},
		{&d.FaultDomain, flagOciFaultDomain},
		{&d.Shape, flagOciShape},
	}
	for _, f := range flags {
		*f.target = fl.String(f.flag)
		if *f.target == "" {
			return requiredOptionError(f.flag)
		}
	}

	log.Debug("Set configuration from flags.")

	return nil
}

// PreCreateCheck validates if driver values are valid to create the machine.
func (d *Driver) PreCreateCheck() (err error) {

	// Validate if firewall rules can be read correctly

	// Check if virtual machine exists. An existing virtual machine cannot be updated.

	// NOTE(ahmetalpbalkan) we could have done more checks here but Oci often
	// returns meaningful error messages and it would be repeating the backend
	// logic on the client side. Some examples:
	//   - Deployment of a machine to an existing Virtual Network fails if
	//     virtual network is in a different region.
	//   - Changing IP Address space of a subnet would fail if there are machines
	//     running in the Virtual Network.
	log.Info("Completed machine pre-create checks.")
	return nil
}

// Create creates the virtual machine.
func (d *Driver) Create() error {
	fmt.Println("Prepareing launching request.")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	fmt.Println("New compute client success, err:", err)
	helpers.FatalIfError(err)
	ctx := context.Background()
	request := core.LaunchInstanceRequest{}

	request.DisplayName = &(d.DisplayName)
	request.AvailabilityDomain = &(d.AvailabilityDomain)
	request.Shape = &(d.Shape)
	fmt.Println("Before get compartmentID")
	compartmentID, err := d.getCompartmentID(ctx, common.DefaultConfigProvider(), d.CompartmentName)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	request.CompartmentId = compartmentID

	imageid, err := d.getImageID(ctx, common.DefaultConfigProvider(), d.ImageName)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	request.ImageId = imageid

	request.RequestMetadata = helpers.GetRequestMetadataWithDefaultRetryPolicy()

	createResp, err := c.LaunchInstance(ctx, request)
	fmt.Println("Launching Oci instance.")

	// should retry condition check which returns a bool value indicating whether to do retry or not
	// it checks the lifecycle status equals to Running or not for this case
	shouldRetryFunc := func(r common.OCIOperationResponse) bool {
		if converted, ok := r.Response.(core.GetInstanceResponse); ok {
			return converted.LifecycleState != core.InstanceLifecycleStateRunning
		}
		return true
	}
	// create get instance request with a retry policy which takes a function
	// to determine shouldRetry or not
	pollingGetRequest := core.GetInstanceRequest{
		InstanceId:      createResp.Instance.Id,
		RequestMetadata: helpers.GetRequestMetadataWithCustomizedRetryPolicy(shouldRetryFunc),
	}
	_, pollError := c.GetInstance(ctx, pollingGetRequest)
	helpers.FatalIfError(pollError)

	fmt.Println("Oci instance launched")

	return nil
}

// Remove deletes the virtual machine and resources associated to it.
func (d *Driver) Remove() error {
	// NOTE In Oci, there is no remove option for virtual
	// machines, terminate is the closest option.
	log.Debug("Oci does not implement remove. Calling terminate instead.")
	request := core.TerminateInstanceRequest{
		RequestMetadata: helpers.GetRequestMetadataWithDefaultRetryPolicy(),
	}
	request.InstanceId = &(d.InstanceID)

	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()
	_, err = c.TerminateInstance(ctx, request)
	helpers.FatalIfError(err)

	fmt.Println("terminating instance")

	// should retry condition check which returns a bool value indicating whether to do retry or not
	// it checks the lifecycle status equals to Terminated or not for this case
	shouldRetryFunc := func(r common.OCIOperationResponse) bool {
		if converted, ok := r.Response.(core.GetInstanceResponse); ok {
			return converted.LifecycleState != core.InstanceLifecycleStateTerminated
		}
		return true
	}

	pollGetRequest := core.GetInstanceRequest{
		RequestMetadata: helpers.GetRequestMetadataWithCustomizedRetryPolicy(shouldRetryFunc),
	}

	pollGetRequest.InstanceId = &(d.InstanceID)
	_, pollErr := c.GetInstance(ctx, pollGetRequest)
	helpers.FatalIfError(pollErr)
	fmt.Println("instance terminated")
	return err
}

// GetIP returns public IP address or hostname of the machine instance.
func (d *Driver) GetIP() (string, error) {

	log.Debugf("Machine IP address resolved to: %s", &(d.resolvedIP))
	return d.resolvedIP, nil
}

// GetSSHHostname returns an IP address or hostname for the machine instance.
func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

// GetURL returns a socket address to connect to Docker engine of the machine
// instance.
func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		return "", err
	}

	// That this is not used until machine is
	// actually created and provisioned. By then GetIP() should be returning
	// a non-empty IP address as the VM is already allocated and connected to.
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	u := (&url.URL{
		Scheme: "tcp",
		Host:   net.JoinHostPort(ip, fmt.Sprintf("%d", d.DockerPort)),
	}).String()
	log.Debugf("Machine URL is resolved to: %s", u)
	return u, nil
}

// Restart reboots the virtual machine instance.
func (d *Driver) Restart() error {
	fmt.Println("Restarting instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId = &(d.InstanceID)
	request.Action = core.InstanceActionActionEnum("SOFTRESET")

	_, err = c.InstanceAction(ctx, request)
	helpers.FatalIfError(err)

	return err
}

// Kill stops the virtual machine role instance.
func (d *Driver) Kill() error {
	// NOTE In Oci, there is no kill option for virtual
	// machines, Stop() is the closest option.
	log.Debug("Oci does not implement kill. Calling Stop instead.")
	return d.Stop()
}

//
func (d *Driver) listCompartments(ctx context.Context, c identity.IdentityClient, compartmentID *string) ([]identity.Compartment, error) {
	request := identity.ListCompartmentsRequest{
		CompartmentId: compartmentID,
	}

	r, err := c.ListCompartments(ctx, request)
	helpers.FatalIfError(err)

	return r.Items, err
}

func (d *Driver) getCompartmentID(ctx context.Context, provider common.ConfigurationProvider, compartmentName string) (*string, error) {
	c, clerr := identity.NewIdentityClientWithConfigurationProvider(provider)
	if clerr != nil {
		fmt.Println("Error:", clerr)
		return nil, clerr
	}
	Compartments, _ := d.listCompartments(ctx, c, &(d.tenancy))

	for _, compartment := range Compartments {
		if *compartment.Name == compartmentName {
			// VCN already created, return it
			return compartment.Id, nil
		}
	}
	err := fmt.Errorf("Can't find Compartment with name %s", compartmentName)
	return nil, err
}

// ListImages lists the available images in the specified compartment.
func (d *Driver) listImages(ctx context.Context, c core.ComputeClient, compartmentID *string) ([]core.Image, error) {
	request := core.ListImagesRequest{
		CompartmentId: compartmentID,
	}

	r, err := c.ListImages(ctx, request)
	helpers.FatalIfError(err)

	return r.Items, err
}

func (d *Driver) getImageID(ctx context.Context, provider common.ConfigurationProvider, imageName string) (*string, error) {
	c, clerr := core.NewComputeClientWithConfigurationProvider(provider)
	if clerr != nil {
		fmt.Println("Error:", clerr)
	}
	Images, _ := d.listImages(ctx, c, &(d.tenancy))

	for _, image := range Images {
		if *image.DisplayName == imageName {
			// VCN already created, return it
			return image.Id, nil
		}
	}
	err := fmt.Errorf("Can't find Image with name %s", imageName)
	return nil, err
}
