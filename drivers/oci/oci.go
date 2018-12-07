package oci

import (
	"github.com/docker/machine/libmachine/drivers"
	"github.com/oracle/oci-go-sdk/common"
)

type Driver struct {
	*drivers.BaseDriver
	InstanceID string
}

const (
	driverName               = "oci"
	ipRange                  = "0.0.0.0/0"
	machineSecurityGroupName = "rancher-nodes"
	machineTag               = "rancher-nodes"
)

//
func (d *Driver) Stop() error  {
	fmt.Println("Stoping inst03")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId=d.InstanceID
	request.Action=core.InstanceActionActionEnum("STOP")

	_, err = c.InstanceAction(ctx , request) 
	helpers.FatalIfError(err)

	return 
}

// 
func (d *Driver) Start() error  {
	fmt.Println("Starting instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId=d.InstanceID
	request.Action=core.InstanceActionActionEnum("START")

	_, err = c.InstanceAction(ctx , request) 
	helpers.FatalIfError(err)

	return err
}
//
func (d *Driver) GetState() (state.State, error) {
	fmt.Println("Getting state of instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.GetInstanceRequest{}
	equest.InstanceId=d.InstanceID

	response, err := c.GetInstance(ctx , request) 
	machineState := response.Instance.LifecycleState
	fmt.Println(response.Instance.LifecycleState)
	helpers.FatalIfError(err)

	return machineState, nil
}