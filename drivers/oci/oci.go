package oci

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"path"

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
	defaultSSHUser            = "opc"
	defaultOciAvailableDomain = "eXkP:PHX-AD-2"
	defaultOciShape           = "VM.Standard2.1"
	defaultOciFaultDomain     = "FAULT-DOMAIN-1"
	defaultOciImageName       = "Oracle-Linux-7.5-2018.10.16-0"
	//defaultOciVCNName         = "OCI-GOSDK-Sample-VCN"
	//defaultOciSubnetName      = "OCI-GOSDK-Sample-Subnet2"
	defaultOciVCNName         = "vcn20190102161726"
	defaultOciSubnetName      = "Public Subnet eXkP:PHX-AD-1"
	defaultOciCompartmentName = "Arancher"
)

const (
	flagOciAvailableDomain = "oci-available-domain"
	flagOciFaultDomain     = "oci-fault-domain"
	flagOciShape           = "oci-shape"
	flagOciImageName       = "oci-image"
	flagOciVCNName         = "oci-vnet"
	flagOciSubnetName      = "oci-subnet"
	flagOciCompartmentName = "oci-compartment"
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
	fmt.Println("Getting state of Oci instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.GetInstanceRequest{}
	request.InstanceId = &(d.InstanceID)
	fmt.Println("request.InstanceId", *(request.InstanceId))

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
		mcnflag.StringFlag{
			Name:   flagOciCompartmentName,
			Usage:  "The display name of compartment",
			EnvVar: "OCI_COMPARTMENT_NAME",
			Value:  defaultOciCompartmentName,
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
		{&d.CompartmentName, flagOciCompartmentName},
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

	homeFolder := getHomeFolder()
	fmt.Println("homeFolder:", homeFolder)

	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	helpers.FatalIfError(err)
	ctx := context.Background()
	request := core.LaunchInstanceRequest{}

	//request.DisplayName = &(d.DisplayName)
	request.DisplayName = common.String("DM-Sample-Instance")
	fmt.Println("#####request.DisplayName:", *(request.DisplayName))
	request.AvailabilityDomain = &(d.AvailabilityDomain)
	fmt.Println("#####request.AvailabilityDomain:", *(request.AvailabilityDomain))
	request.Shape = &(d.Shape)
	fmt.Println("#####request.Shape:", *(request.Shape))
	subnetid := "ocid1.subnet.oc1.phx.aaaaaaaalczycwrl45llhmqqibdqgz4ddetkg6uvmpjl27i5dw5wzsiac6eq"
	request.SubnetId = &subnetid
	fmt.Println("#####request.SubnetId:", *(request.SubnetId))
	fmt.Println("#####Before get compartmentID")
	fmt.Println("#####CompartmentName:", d.CompartmentName)
	compartmentID, err := d.getCompartmentID(ctx, common.DefaultConfigProvider(), d.CompartmentName)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	fmt.Println("#####compartmentID:", *compartmentID)

	request.CompartmentId = compartmentID
	fmt.Println("#####request.CompartmentId:", *(request.CompartmentId))

	imageid, err := d.getImageID(ctx, common.DefaultConfigProvider(), d.ImageName)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	fmt.Println("imageid:", *imageid)

	request.ImageId = imageid
	fmt.Println("#####request.ImageId:", *(request.ImageId))
	request.RequestMetadata = helpers.GetRequestMetadataWithDefaultRetryPolicy()

	metadata := map[string]string{
		"user_data":           "dW5kZWZpbmVk",
		"ssh_authorized_keys": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDd25ZgCEms2Cnt922S4PZVQolmvPDLJsWG8dAGEijlqPh7vepzJvCayaIymU6C6DEDtAqRN/CPm6tcIG/TFvy4al9pseIXAngfPfwNoC1jYdBYM941cEt2legcmkBCoB/wIK69SefRbO3nfbLxh/2ebtRWTJey5658wUS3JODoE9wd22EAg87I0P2Fbpo1W3kVZqF+cj7x0+t1ewZ4Rg2Bf98+hs9U9JmnmgPdk7cpo9CfF6FoiSRMMWb1kxaqESP8Q/gleajk6g1GZQkE7hEy9OxwI1QpLaAy/557vD/wJ5C0di9h+dA5gYe0QXeBeZ6zPlllJhilWehPtJIfT5hC57ks9+fBwZPqNwE92lICq5tiU8PfpamqRb1F1KiPN88G2fNUKGJHejN5DziKw6b4+RzzLneRv5VtK/FGm9wPGRdRhLzi7Wk59um9NDvd63GDV5ebQCjYBOGd1B82S9bpZlSHoewWXL9yavL5un5X8+/fETXlUkkKB4DRuKU6/aSbe0tKynngY0ZsdyJ/OcS1UbibOAXrt/AYl2/g15gWFYIRvm7VC20immiT4wf1B2fi87o5fbHfWuViJsxhjG4Eb1/0rTkJCTPV8RnNnjiKUJ9k7SRsw+NaK88MNFye0E7sTvl3Z+5vcuKZRatSVdRuP0XztvfyjXmlx2goM/dWMw== jet_sample_ww_grp@oracle.com",
	}

	request.Metadata = metadata

	createResp, err := c.LaunchInstance(ctx, request)

	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
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

	d.InstanceID = *(createResp.Instance.Id)
	fmt.Println("Oci instance launched")

	d.getIPs()

	return nil
}

// Remove deletes the virtual machine and resources associated to it.
func (d *Driver) Remove() error {
	return nil
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

	log.Debugf("OCI Machine IP address resolved to:", &(d.resolvedIP))
	fmt.Println("OCI Machine IP address resolved to:", d.resolvedIP)
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

	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	helpers.FatalIfError(err)

	return r.Items, err
}

func (d *Driver) getCompartmentID(ctx context.Context, provider common.ConfigurationProvider, compartmentName string) (*string, error) {
	fmt.Println("inside getCompartmentID")
	c, clerr := identity.NewIdentityClientWithConfigurationProvider(provider)
	if clerr != nil {
		fmt.Println("Error:", clerr)
		return nil, clerr
	}

	fmt.Println("NewIdentityClientWithConfigurationProvider success")
	tt := "ocid1.tenancy.oc1..aaaaaaaashw7efstoxf6v46gevtascttepw3l3d6xxx4gziexn5sxnldyhja"
	//Compartments, lerr := d.listCompartments(ctx, c, &(d.tenancy))
	Compartments, lerr := d.listCompartments(ctx, c, &tt)
	if lerr != nil {
		fmt.Println("Error:", lerr)
		return nil, lerr
	}

	fmt.Println("listCompartments success")

	for _, compartment := range Compartments {
		//fmt.Println("compartment:", *(compartment.Name))
		if *(compartment.Name) == compartmentName {
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
	fmt.Println("inside getImageID")
	c, clerr := core.NewComputeClientWithConfigurationProvider(provider)
	if clerr != nil {
		fmt.Println("Error:", clerr)
	}
	tt := "ocid1.tenancy.oc1..aaaaaaaashw7efstoxf6v46gevtascttepw3l3d6xxx4gziexn5sxnldyhja"
	//Images, _ := d.listImages(ctx, c, &(d.tenancy))
	Images, _ := d.listImages(ctx, c, &tt)

	for _, image := range Images {
		//fmt.Println("image name:", *(image.DisplayName))
		if *(image.DisplayName) == imageName {
			// VCN already created, return it
			return image.Id, nil
		}
	}
	err := fmt.Errorf("Can't find Image with name %s", imageName)
	return nil, err
}

func getHomeFolder() string {
	current, e := user.Current()
	if e != nil {
		//Give up and try to return something sensible
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return current.HomeDir
}

// getIPs returns public IP address or hostname of the machine instance.
func (d *Driver) getIPs() {
	fmt.Println("Start to get private ip")
	ctx := context.Background()

	// Create compute client to manipulate instance
	cc, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)

	// Step 1: List attached vnic for specified instance
	req1 := core.ListVnicAttachmentsRequest{}

	compartmentID, err := d.getCompartmentID(ctx, common.DefaultConfigProvider(), d.CompartmentName)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("#####compartmentID:", *compartmentID)

	req1.CompartmentId = compartmentID
	req1.InstanceId = &(d.InstanceID)
	//req1.CompartmentId = common.String("ocid1.tenancy.oc1..aaaaaaaashw7efstoxf6v46gevtascttepw3l3d6xxx4gziexn5sxnldyhja")
	//req1.InstanceId = common.String("ocid1.instance.oc1.phx.abyhqljsxxlqrfzioipa4vrr2bwjqxxnmzjidh2kayp4yvzqrw6lyrqkzczq")

	resp1, err := cc.ListVnicAttachments(ctx, req1)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Step 2: Get ocid of vnic pos 0
	vnicID := resp1.Items[0].VnicId

	// Step 3: Get vnic by ocid
	vnc, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	req2 := core.GetVnicRequest{}
	req2.VnicId = vnicID
	resp2, err := vnc.GetVnic(ctx, req2)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Step 4: Get private ip & public ip from vnic
	privateIP := resp2.PrivateIp
	publicIP := resp2.PublicIp

	fmt.Println("Private ip is: ", *privateIP)
	fmt.Println("Public ip is: ", *publicIP)

	d.resolvedIP = *publicIP
}

// GetSSHKeyPath returns the ssh key path
func (d *Driver) GetSSHKeyPath() string {

	if d.SSHKeyPath == "" {
		homeFolder := getHomeFolder()
		d.SSHKeyPath = path.Join(homeFolder, ".oci", "id_rsa_jet")
	}
	fmt.Println("SSHKeyPath is: ", d.SSHKeyPath)
	return d.SSHKeyPath

}

// GetSSHUsername returns the ssh user name, opc if not specified
func (d *Driver) GetSSHUsername() string {
	if d.SSHUser == "" {
		d.SSHUser = "opc"
	}
	return d.SSHUser
}
