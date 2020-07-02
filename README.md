# Simple Container Executor

This project intends to provide a simple method to contain a linux process, by bounding its consumption of certain resource dimensions (ex CPU, VSZ), and limiting its access to certain facilities of the host OS (ex filesystem).

**Note:** This code was **circa 2016**, and **has NOT been updated to modern best practices**! This is just saved for posterity, and not intended to be a reflection of anything modern or reasonable! **You have been warned**

## Problem Statement

With the rapid increase in main memory and cpu core counts on modern machines, it is often useful to partition the resources of a single machine among many tasks. Please write a linux program that will execute another program in a constrained environment. The constraints it should support are:

### chroot

The contained program’s view of the file system should be limited to a specified directory

### memory-limit

If the contained program exceeds this limit it should be killed

### cpu-core-ids

The set of cpu cores that the isolated program may be scheduled on

### network-address

The isolated program should only see a single network interface with the given ip address and subnet. Additionally, two different constrained programs on the same host in the same subnet should be able to communicate over IP.

### Output

This program should be transparent in that the exit code should match that of the contained program, signals should be propagated, etc.

Here is an example of how such a program might be invoked:

```
$ ./bin/contain ­­chroot /var/run/mytask \
­­--memory-limit 1MB \
­­--cpu-core­ids 1,3 \
­­--network-address 192.168.2.17/24 \
­ -- someprogram arg1 arg
```

## Constraints

* CPU Cores - Restrict execution of the program to only specific core IDs via cgroups
* Virtual Size - Restrict virtual memory allocation to not exceed limit via setrlimit/cgroups
* Chroot - Execute process within a chroot
* Network - Give process a separate network namespace

# Build

Requires:
* Go 1.4+
* Godep (make setup will install for you)

## Get Started

Place this somewhere in your GOPATH. I put it under my github user (dont worry, its not on github!)

```
$ mkdir -p $GOPATH/src/github.com/byxorna
$ tar xzvf homework.tar.gz -C $GOPATH/src/github.com/byxorna
$ cd $GOPATH/src/github.com/byxorna/simple-container-executor
```

## Setup deps

```
$ make setup
-> install build deps
```

## Build the thing

```
$ make
-> go fmt
-> go vet
-> go test
?       github.com/byxorna/simple-container-executor    [no test files]
-> go build
$ ./contain -h
Usage of ./contain -chroot $ROOTFS [optflags] program [...args]
Version: f25daba-hacky Branch: master Commit: f25daba
  -bridge string
        Bridge interface to hook veth devices up to (must have same CIDR range as -network-address) (default "docker0")
  -chroot string
        Path to chroot the process (required)
  -cpu-core-ids string
        List of CPU core IDs (i.e. "0,1,2", "0-7", "3,5-7")
  -memory-limit value
        Virtual memory size limit (i.e. 128MB)
  -network-address string
        Interface address (CIDR format) for network namespace isolation
```

# Run

## Setup a rootfs first

You need a fully populated rootfs for your contained process. Set up a minimal base. This example works if you are on archlinux:

```
# for i in 1 2 ; do
	curl https://raw.githubusercontent.com/tokland/arch-bootstrap/master/arch-bootstrap.sh -o ./arch-bootstrap.sh
	chmod +x arch-bootstrap.sh
	rootfs="$(mktemp -d)"
	./arch-bootstrap.sh $rootfs
	# install any other stuff you want in your chroot
	pacman -Sy -r $rootfs iproute2 iputils net-tools
	export ROOTFS_$i="${rootfs}"
done
```

## Launch a process
Run 2 instances and show that they can ping eachother :)
```
# ./contain -chroot $ROOTFS_1 --network-address 172.17.0.99/24 --bridge docker0 -- bash -c 'ip a ; ping -c 100 172.17.0.98'
# ./contain -chroot $ROOTFS_2 --cpu-core-ids 1,2 --memory-limit 128m --network-address 172.17.0.98/24 --bridge docker0 -- bash -c 'ip a ; ip ro sho 172.17.0.99/0 ; ping -c 5 172.17.0.99'
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default 
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
165: eth0@if166: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether 00:00:00:ab:f2:7f brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 172.17.0.98/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::200:ff:feab:f27f/64 scope link tentative 
       valid_lft forever preferred_lft forever
default via 172.17.0.1 dev eth0 
PING 172.17.0.99 (172.17.0.99) 56(84) bytes of data.
64 bytes from 172.17.0.99: icmp_seq=1 ttl=64 time=0.333 ms
64 bytes from 172.17.0.99: icmp_seq=2 ttl=64 time=0.096 ms
64 bytes from 172.17.0.99: icmp_seq=3 ttl=64 time=0.160 ms
64 bytes from 172.17.0.99: icmp_seq=4 ttl=64 time=0.168 ms
64 bytes from 172.17.0.99: icmp_seq=5 ttl=64 time=0.166 ms

--- 172.17.0.99 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 3998ms
rtt min/avg/max/mdev = 0.096/0.184/0.333/0.080 ms

```

# Assumptions

## General

* You are ok with using the native golang flags package, which does not provide a longopt (i.e. `--myopt`) form. Thus, `--cpu-core-ids` should be given as `-cpu-core-ids`).
  * I like to keep external deps to a minimum, and tend to use facilities built into the language/framework/whatever rather than use something outside of mainline
  * It tends to be more maintainable, supportable, and reliable.
  * Golang mainline packages tend to be simple and minimal. Complexity is the enemy of distributed systems :D
  * It actually appears that golang 1.5+ `flags` will parse longopts. So, if you build with 1.5, you can use `--cpu-core-ids` like the homework shows in the example

## Environment

* /var/lib/container is where you want libcontainer to keep metadata about your containers - its a pretty reasonable assumption
* You have root access on the host you are running this
  * /var/lib/container is root:root 600 generally, and you need lots of CAPs to spawn containers and change namespaces
* You need a full OS install in your chroot - i.e. your executable and all transitive dependencies.
  * This is the same assumption chroot(1) makes (as well as libcontainer :P)
  * A simple method to setup a rootfs is arch-chroot (https://github.com/tokland/arch-bootstrap)

## Execution

* `-cpu-core-ids` is the same format allowed by cgroup's cpuset.cpus (i.e. 1,2,3 || 0-7)
* Omitting `-cpu-core-ids` will assume you want to place no limitation on which CPUs your program executes
* The PATH for the contained process is fixed to /bin:/usr/bin:/sbin:/usr/sbin
* Omitting `-memory-limit` will assume you want no virtual size restrictions on your program execution
* Omitting the `-network-address` flag will still isolate the network in its own namespace, but only give you a loopback interface
* Your veth pair will always have the mac address with the OUI (3 MSB) zeroed out. Should be fine, i think
* The default bridge interface that your veth will get hooked up to is "docker0". You can override with `--bridge`.
* the IP you specify with `-network-address` must be within the CIDR of your bridge interface, otherwise the container wont launch (network is unreachable)
* Your bridge interface is already configured! This is easy if you are running this on the same host as docker, so you can just reuse `docker0`.
* Your bridge interface should have an IPv4 address on it. In my testing, libcontainer is unhappy if you give it an IPv6 gateway for a veth interface

# Design Decisions

I chose to base this on `runc/libcontainer` (https://github.com/opencontainers/runc/blob/master/libcontainer/), because it is the newly minted library for running containers on Linux. It is used by both `rkt` and `docker` as an interoperable standard, and should provide much of the required functionality, while being reasonably futureproof. This provides required functionality (i.e. chroot, namespaces, cgroups) in a more well vetted and tested interface than if I tried to use all the low level interfaces myself. :P (Work smarter, not harder).

Go is shaping up to be the language of containers on Linux. Practically all new container-focused development is happening in Go. Docker, rkt, libcontainer, etcd, coreos, and many other industry leading projects are written in Go, which encourages tooling around that domain to become specialized and more well developed, which creates a virtuous cycle.

I chose Godeps as a vendored library manager was chosen because it has a low cognitive overhead, and I am familiar with it. It gets out of your way, and works pretty well. I chose to not use the new Go 1.5 vendor experiment, because it would tie me to 1.5, and I haven't found the tooling around it to be very sublime (always searching for the simplicity of `bundle install`). Godep strikes a happy balance for me between providing necessary functionality missing in the core language (vendored dep management) and ease of setup and use, while not prescribing too much about how you work.

I also chose to explicitly NOT do any management of bridge interfaces. While yes, the assignment could be interpreted that the `contain` program should perform bridge interface creation, I chose to do the least amount of munging around on the host and require that the user already has set up the bridge interfaces with IP addresses that you want your programs to communicate on.

# Gotchas

* If you ctrl-c the process and you spawned bash or sh in your container, bash/sh wont propogate signals to children. Thus, it will look like running `bash -c 'ping google.com'` will never terminate. I didnt implement `-it` like in docker :P
* I try to find an IPv4 address on the bridge, and use that as the gateway for the veth in the container. Things wont work if you only have an IPv6 interface on the bridge (is this a limitation of libcontainer?).

