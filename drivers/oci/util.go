package oci

import (
	"fmt"
	"github.com/docker/machine/libmachine/state"
	"github.com/oracle/oci-go-sdk/core"
	"github.com/docker/machine/libmachine/log"
)

type requiredOptionError string

func (r requiredOptionError) Error() string {
	return fmt.Sprintf("%s driver requires the %q option.", driverName, string(r))
}

func machineStateForLifecycleState(ls core.InstanceLifecycleStateEnum) state.State {
	m := map[core.InstanceLifecycleStateEnum]state.State{
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