package oci

import (
	"fmt"
	"strings"

	"github.com/docker/machine/drivers/driverutil"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	"github.com/oracle/oci-go-sdk/core"
)

type requiredOptionError string

func (r requiredOptionError) Error() string {
	return fmt.Sprintf("%s driver requires the %q option.", driverName, string(r))
}

func machineStateForLifecycleState(ls core.InstanceLifecycleStateEnum) state.State {
	m := map[core.InstanceLifecycleStateEnum]state.State{
		core.InstanceLifecycleStateRunning:       state.Running,
		core.InstanceLifecycleStateStarting:      state.Starting,
		core.InstanceLifecycleStateProvisioning:  state.Starting,
		core.InstanceLifecycleStateCreatingImage: state.Starting,
		core.InstanceLifecycleStateStopping:      state.Stopping,
		core.InstanceLifecycleStateTerminating:   state.Stopping,
		core.InstanceLifecycleStateStopped:       state.Stopped,
		core.InstanceLifecycleStateTerminated:    state.Stopped,
	}

	if v, ok := m[ls]; ok {
		return v
	}
	log.Warnf("Oci LifecycleState %q does not map to a docker-machine state.", ls)
	return state.None
}

func GetSSHClientFromDriver(d Driver) (ssh.Client, error) {
	address, err := d.GetSSHHostname()
	if err != nil {
		return nil, err
	}

	port, err := d.GetSSHPort()
	if err != nil {
		return nil, err
	}

	var auth *ssh.Auth
	if d.GetSSHKeyPath() == "" {
		auth = &ssh.Auth{}
	} else {
		auth = &ssh.Auth{
			Keys: []string{d.GetSSHKeyPath()},
		}
	}

	client, err := ssh.NewClient(d.GetSSHUsername(), address, port, auth)
	return client, err

}

func RunSSHCommandFromDriver(d Driver, command string) (string, error) {
	client, err := GetSSHClientFromDriver(d)
	if err != nil {
		return "", err
	}

	log.Debugf("About to run SSH command:\n%s", command)

	output, err := client.Output(command)
	log.Debugf("SSH cmd err, output: %v: %s", err, output)
	if err != nil {
		return "", fmt.Errorf(`ssh command error:
command : %s
err     : %v
output  : %s`, command, err, output)
	}

	return output, nil
}

func RunMultiSSHCommandFromDriver(d Driver, commands []string) (string, error) {
	client, err := GetSSHClientFromDriver(d)
	if err != nil {
		return "", err
	}

	log.Debugf("About to run SSH command:\n%s", commands)

	for _, cmd := range commands {
		output, err := client.Output(cmd)
		log.Debugf("SSH cmd err, output: %v: %s", err, output)
		if err != nil {
			return "", fmt.Errorf(`ssh command error:
	command : %s
	err     : %v
	output  : %s`, cmd, err, output)
		}
	}

	return "succeed", nil
}

func WaitForSSH(d Driver) error {
	// Try to dial SSH for 30 seconds before timing out.
	if err := mcnutils.WaitFor(sshAvailableFunc(d)); err != nil {
		return fmt.Errorf("Too many retries waiting for SSH to be available.  Last error: %s", err)
	}
	return nil
}

func sshAvailableFunc(d Driver) func() bool {
	return func() bool {
		log.Debug("Getting to WaitForSSH function...")
		if _, err := RunSSHCommandFromDriver(d, "exit 0"); err != nil {
			log.Debugf("Error getting ssh command 'exit 0' : %s", err)
			return false
		}
		return true
	}
}

func ConfigIPtables(d Driver) (string, error) {
	fmt.Println("Configuring iptable .. ")
	WaitForSSH(d)

	preCommands := []string{
		"pwd",
		"sudo iptables-save > $HOME/firewall.txt",
		"sed -i \"/--dport 22/a\\-A INPUT -p tcp -m state --state NEW -m tcp --dport 2376 -j ACCEPT\" firewall.txt",
	}
	portCommands := []string{}
	afterCommands := []string{
		"sudo iptables-restore < $HOME/firewall.txt",
		"sudo ufw reload",
		"sudo ufw enable",
	}

	command := "sed -i \"/--dport 22/a\\-A INPUT -p protocal -m state --state NEW -m protocal --dport port -j ACCEPT\" firewall.txt"

	d.OpenPorts = []string{"6443/tcp", "2379/tcp", "2380/tcp", "8472/udp", "4789/udp", "10256/tcp", "10250/tcp", "10251/tcp", "10252/tcp"}

	for _, p := range d.OpenPorts {
		port, protocol := driverutil.SplitPortProto(p)
		fmt.Println(port, protocol)
		c1 := strings.Replace(command, "protocal", protocol, -1)
		c2 := strings.Replace(c1, " port", " "+port, -1)
		fmt.Println(c2)
		portCommands = append(portCommands, c2)
	}

	commands := append(preCommands, portCommands...)
	commands = append(commands, afterCommands...)

	output, err := RunMultiSSHCommandFromDriver(d, commands)

	if err != nil {
		fmt.Printf("run ssh command failed: %s\n", err)
		return "", err
	}
	fmt.Printf("config iptable to expose port 2376 succeed: %s\n", output)
	return output, nil
}

func ConfigFirewalld(d Driver) (string, error) {
	fmt.Println("Configuring firewall .. ")
	WaitForSSH(d)

	preCommands := []string{
		"sudo firewall-cmd --zone=public --add-port=2376/tcp --permanent",
	}
	portCommands := []string{}
	afterCommands := []string{
		"sudo firewall-cmd --reload",
	}

	command := "sudo firewall-cmd --zone=public --add-port=portproto --permanent"

	d.OpenPorts = []string{"6443/tcp", "2379/tcp", "2380/tcp", "8472/udp", "4789/udp", "10256/tcp", "10250/tcp", "10251/tcp", "10252/tcp"}

	for _, p := range d.OpenPorts {
		c1 := strings.Replace(command, "portproto", p, -1)
		fmt.Println(c1)
		portCommands = append(portCommands, c1)
	}

	commands := append(preCommands, portCommands...)
	commands = append(commands, afterCommands...)

	output, err := RunMultiSSHCommandFromDriver(d, commands)

	if err != nil {
		fmt.Printf("run ssh command failed: %s\n", err)
		return "", err
	}
	fmt.Printf("config firewall to expose port 2376 succeed: %s\n", output)
	return output, nil
}
