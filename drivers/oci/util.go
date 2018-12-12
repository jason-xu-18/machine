package oci

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/docker/machine/libmachine/state"
	"github.com/oracle/oci-go-sdk/core"
)

func machineStateForLifecycleState(ls InstanceLifecycleStateEnum) state.State {
	m := map[InstanceLifecycleStateEnum]state.State{
		core.InstanceLifecycleStateRunning:     state.Running,
		core.InstanceLifecycleStateStarting:    state.Starting,
		core.InstanceLifecycleStateProvisioning:    state.Starting,
		core.InstanceLifecycleStateCreatingImage:    state.Starting,
		core.InstanceLifecycleStateStopping:    state.Stopping,
		core.InstanceLifecycleStateTerminating:    state.Stopping,
		core.InstanceLifecycleStateStopped:    state.Stopped,
		core.InstanceLifecycleStateTerminated:  state.Stopped,
		
	}

	if v, ok := m[ls]; ok {
		return v
	}
	log.Warnf("Oci LifecycleState %q does not map to a docker-machine state.", ls)
	return state.None
}