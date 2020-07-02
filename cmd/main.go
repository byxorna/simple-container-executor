package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/byxorna/simple-container-executor/pkg/util"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
)

type CliOptions struct {
	MemoryLimit     bytesFmt // memory limit in bytes
	CpuSetCpus      string   // cpu core IDs, same format as in https://www.kernel.org/doc/Documentation/cgroups/cpusets.txt cpuset.cpus
	Rootfs          string   // chroot path on host
	NetworkAddress  string   // network address (i.e. 10.0.10.5/24)
	BridgeInterface string   // interface of bridge
	Program         []string // executable and args
}

const (
	CONTAINER_HOST_PATH = "/var/lib/container"
)

var (
	opts CliOptions

	// set by makefile with -X main.<varname> on build
	branch  = "???"
	commit  = "???"
	version = "???"
)

func Fatal(e error) {
	fmt.Printf("Error: %s\n", e.Error())
	os.Exit(1)
}

func init() {

	if len(os.Args) > 1 && os.Args[1] == "init" {
		// act as a parent for a container
		// This is copypasta from https://github.com/opencontainers/runc/blob/8b195816941e4c6e5e9591bcf7b8fbdbf106cd01/start.go#L67-L77
		// I dont pretend to understand the complexities of LockOSThread() :P
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, err := libcontainer.New("")
		if err != nil {
			Fatal(errors.New("unable to initialize for container: " + err.Error()))
		}

		if err := factory.StartInitialization(); err != nil {
			Fatal(errors.New("Init: " + err.Error()))
		}
		panic("--this line should have never been executed, congratulations--")
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s -chroot $ROOTFS [optflags] program [...args]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Version: %s Branch: %s Commit: %s\n", version, branch, commit)
		flag.PrintDefaults()
	}

	flag.Var(&opts.MemoryLimit, "memory-limit", "Virtual memory size limit (i.e. 128MB)")
	flag.StringVar(&opts.Rootfs, "chroot", "", "Path to chroot the process (required)")
	flag.StringVar(&opts.CpuSetCpus, "cpu-core-ids", "", `List of CPU core IDs (i.e. "0,1,2", "0-7", "3,5-7")`)
	flag.StringVar(&opts.NetworkAddress, "network-address", "", "Interface address (CIDR format) for network namespace isolation")
	flag.StringVar(&opts.BridgeInterface, "bridge", "docker0", "Bridge interface to hook veth devices up to (must have same CIDR range as -network-address)")
	flag.Parse()

	// any remaining arguments are the program and args to execute
	args := flag.Args()
	if len(args) == 0 {
		Fatal(errors.New("You need to give me some program and arguments to contain!"))
	}
	opts.Program = args

	// input validation
	if opts.Rootfs == "" {
		Fatal(errors.New("You need to provide me a chroot with -chroot"))
	}

}

// loadContainerFactory returns the configured factory instance for execing containers.
// lifted from https://github.com/opencontainers/runc/blob/8b195816941e4c6e5e9591bcf7b8fbdbf106cd01/utils.go#L100-L111
func loadContainerFactory() (libcontainer.Factory, error) {
	return libcontainer.New(CONTAINER_HOST_PATH, libcontainer.Cgroupfs, func(l *libcontainer.LinuxFactory) error {
		return nil
	})
}

func main() {
	//fmt.Printf("Options: %+v\n", opts)

	// lets grab all the interfaces on this box and validate our bridge interface
	// and extract the IPv4 interface from it (to use as our gateway if spinning up a new network namespace
	ifaces, err := net.Interfaces()
	if err != nil {
		Fatal(errors.New("Unable to retrieve interfaces: " + err.Error()))
	}
	bridgeIp := ""
	for _, i := range ifaces {
		if i.Name == opts.BridgeInterface {
			//fmt.Printf("Found %s\n", i.Name)
			addrs, err := i.Addrs()
			if err != nil {
				Fatal(fmt.Errorf("Unable to grab addrs from %s: %s", opts.BridgeInterface, err.Error()))
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				// only accept IPv4 IPs
				if ip4 := ip.To4(); ip4 != nil {
					//fmt.Printf("Got addr: %s\n", ip4.String())
					bridgeIp = ip4.String()
					break
				}
			}
		}
	}

	if bridgeIp == "" {
		Fatal(fmt.Errorf("Unable to find bridge interface %s; are you sure you have this configured as a bridge with an IP on it?", opts.BridgeInterface))
	}
	//fmt.Printf("Found bridge IP: %s\n", bridgeIp)

	//resolve rootfs to an absolute path
	cleanRootFs, err := utils.ResolveRootfs(opts.Rootfs)
	if err != nil {
		Fatal(errors.New(fmt.Sprintf("Unable to resolve root filesystem %s: %s", opts.Rootfs, err.Error())))
	}
	//fmt.Printf("Rootfs: %s\n", cleanRootFs)

	//generate a name for the container
	containerName, err := utils.GenerateRandomName("", 16)
	if err != nil {
		Fatal(err)
	}
	//println("Generated name: " + containerName)

	//fmt.Printf("Loading container factory\n")
	containerFactory, err := loadContainerFactory()
	if err != nil {
		Fatal(err)
	}

	config := &configs.Config{
		NoPivotRoot: true, //  we dont care about pivoting root; just chroot is fine, thank you!
		Rootfs:      cleanRootFs,
		Capabilities: []string{
			"CAP_CHOWN",
			"CAP_DAC_OVERRIDE",
			"CAP_FSETID",
			"CAP_FOWNER",
			"CAP_MKNOD",
			"CAP_NET_RAW",
			"CAP_SETGID",
			"CAP_SETUID",
			"CAP_SETFCAP",
			"CAP_SETPCAP",
			"CAP_NET_BIND_SERVICE",
			"CAP_SYS_CHROOT",
			"CAP_KILL",
			"CAP_AUDIT_WRITE",
		},
		Namespaces: configs.Namespaces([]configs.Namespace{
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWPID},
			{Type: configs.NEWNET},
		}),
		Cgroups: &configs.Cgroup{
			Name:            containerName,
			Parent:          "system",
			AllowAllDevices: false,
			AllowedDevices:  configs.DefaultAllowedDevices,
		},
		Mounts: []*configs.Mount{
			&configs.Mount{Device: "proc", Destination: "/proc"},
			&configs.Mount{Device: "sysfs", Destination: "/sys"},
			&configs.Mount{Device: "tmpfs", Destination: "/dev"}, // we need /dev to be tmpfs so libcontainer can make device nodes
		},
		//Devices: configs.DefaultAutoCreatedDevices, //fuse not readable? wonder what thats about
		Devices:  configs.DefaultSimpleDevices, // if this is present, simple devices are created before pivoting/chrooting
		Hostname: containerName,
		Networks: []*configs.Network{
			{
				Type:    "loopback",
				Address: "127.0.0.1/0",
				Gateway: "localhost",
			},
		},
		/* dont set any rlimits for now. Maybe future version? :)
		Rlimits: []configs.Rlimit{
			{
				Type: syscall.RLIMIT_NOFILE,
				Hard: uint64(1024),
				Soft: uint64(1024),
			},
		}, */
	}

	if opts.MemoryLimit != 0 {
		// isolate cgroup to VSZ memory limit
		config.Cgroups.Memory = int64(opts.MemoryLimit)
	}
	if opts.CpuSetCpus != "" {
		// isolate cpuset to specific cores
		config.Cgroups.CpusetCpus = opts.CpuSetCpus
	}
	if opts.NetworkAddress != "" {
		// linux freaks out if the interface name is longer than 10 characters apparently :P
		hostInterfaceName, err := utils.GenerateRandomName("veth", 6)
		if err != nil {
			Fatal(fmt.Errorf("Unable to generate host interface name: %s", err))
		}
		// and we cant hardcode a mac address for the container side of the veth pair, because
		// then our containers cant talk to eachother because arp will get confused
		mac, err := util.RandomMac()
		if err != nil {
			Fatal(fmt.Errorf("Unable to generate mac address: %s", err))
		}
		config.Networks = append(config.Networks, &configs.Network{
			Type:              "veth",
			Address:           opts.NetworkAddress,
			Bridge:            opts.BridgeInterface,
			HostInterfaceName: hostInterfaceName, // prefix of host interface (will show up as something like vethxyz@if30 on host)
			Mtu:               1500,
			MacAddress:        mac,
			Name:              "eth0",
			Gateway:           bridgeIp,
		})
	}

	//fmt.Printf("Creating container %s...\n", containerName)
	container, err := containerFactory.Create(containerName, config)
	if err != nil {
		Fatal(err)
	}

	// launch the program in the container now
	process := &libcontainer.Process{
		Args:   opts.Program,
		Env:    []string{"PATH=/bin:/usr/bin:/sbin:/usr/sbin"},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// setup signal handling to send a SIGINT and SIGTERM to the process if we capture it
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for {
			s := <-sigc
			// lets just send the signal to the process and not exit until our Wait() returns
			//fmt.Printf("Sending signal %s to %+v\n", s, process)
			err := process.Signal(s)
			if err != nil {
				fmt.Printf("Error sending signal: %s\n", err)
			}
		}
	}()

	err = container.Start(process)
	if err != nil {
		Fatal(errors.New("Unable to launch container: " + err.Error()))
	}

	// destroy the container.
	defer container.Destroy()

	// wait for the process to finish.
	status, _ := process.Wait()
	os.Exit(status.Sys().(syscall.WaitStatus).ExitStatus())

}
