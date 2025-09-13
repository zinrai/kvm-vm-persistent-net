# kvm-vm-persistent-net

A command-line tool to configure persistent network interface names for KVM virtual machines using udev rules.

## Overview

`kvm-vm-persistent-net` simplifies the process of creating persistent network interface names for KVM virtual machines. It works by:

1. Extracting MAC addresses from a KVM virtual machine using `virsh dumpxml`
2. Generating appropriate udev rules to map these MAC addresses to predictable interface names
3. Copying the generated rules file to the virtual machine using `virt-copy-in`

This tool is particularly useful for virtual machines with multiple network interfaces, ensuring they maintain consistent naming across reboots.

## Prerequisites

- KVM/QEMU virtualization environment
- `virsh` command-line tools
- `libguestfs-tools` package (for `virt-copy-in`)
- Sudo privileges

## Installation

```bash
$ go build
```

## Usage

```bash
$ kvm-vm-persistent-net <vm-name>
```

### Command-line Options

```
$ kvm-vm-persistent-net [flags] <vm-name>
```

Available flags:
- `--help`: Display help information
- `--dry-run`: Show the rules file contents without copying to VM
- `--prefix`: Interface name prefix (default: "eth")
- `--start-index`: Starting index for interface numbering (default: 0)
- `--rule-name`: Filename for the udev rules (default: "70-persistent-net.rules")
- `--verbose`: Display verbose output

### Examples

Basic usage with default settings:

```bash
$ kvm-vm-persistent-net centos7-vm
```

Change interface prefix to "enp":

```bash
$ kvm-vm-persistent-net --prefix enp centos7-vm
```

Start interface numbering from 1:

```bash
$ kvm-vm-persistent-net --start-index 1 ubuntu-vm
```

Preview the rules file without copying to VM:

```bash
$ kvm-vm-persistent-net --dry-run debian-vm
```

## License

This project is licensed under the [MIT License](./LICENSE).
