package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// network interface from virsh XML
type Interface struct {
	Mac struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
}

// virsh domain XML structure
type Domain struct {
	Interfaces []Interface `xml:"devices>interface"`
}

func main() {
	// Define command line flags
	help := flag.Bool("help", false, "Display help information")
	dryRun := flag.Bool("dry-run", false, "Show the rules file contents without copying to VM")
	prefix := flag.String("prefix", "eth", "Interface name prefix")
	startIndex := flag.Int("start-index", 0, "Starting index for interface numbering")
	ruleName := flag.String("rule-name", "70-persistent-net.rules", "Filename for the udev rules")
	verbose := flag.Bool("verbose", false, "Display verbose output")

	flag.Parse()

	// Display help if requested
	if *help {
		displayHelp()
		return
	}

	// Check if VM name is provided
	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("Error: VM name is required")
		displayHelp()
		os.Exit(1)
	}
	vmName := args[0]

	if *verbose {
		fmt.Printf("Processing VM: %s\n", vmName)
	}

	// Check if VM exists and is shut off
	if err := checkVMStatus(vmName); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Get VM XML and extract MAC addresses
	macAddresses, err := getMacAddresses(vmName)
	if err != nil {
		fmt.Printf("Error: Failed to get MAC addresses: %v\n", err)
		os.Exit(1)
	}

	if len(macAddresses) == 0 {
		fmt.Println("Warning: No network interfaces found in the VM")
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Found %d network interfaces\n", len(macAddresses))
	}

	// Generate udev rules file
	rulesFile, err := generateRulesFile(macAddresses, vmName, *prefix, *startIndex, *ruleName)
	if err != nil {
		fmt.Printf("Error: Failed to generate rules file: %v\n", err)
		os.Exit(1)
	}

	// Display rules file content
	rulesContent, err := os.ReadFile(rulesFile)
	if err != nil {
		fmt.Printf("Error: Failed to read generated rules file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated udev rules:")
	fmt.Println("----------------------------------------")
	fmt.Println(string(rulesContent))
	fmt.Println("----------------------------------------")

	// If dry-run, exit here
	if *dryRun {
		fmt.Println("Dry run completed. Rules file not copied to VM.")
		os.Remove(rulesFile)
		return
	}

	// Copy rules file to VM
	if err := copyRulesToVM(rulesFile, vmName, *ruleName); err != nil {
		fmt.Printf("Error: Failed to copy rules to VM: %v\n", err)
		os.Remove(rulesFile)
		os.Exit(1)
	}

	// Clean up and display completion message
	os.Remove(rulesFile)
	fmt.Printf("Successfully configured network interfaces for VM '%s'\n", vmName)
	fmt.Printf("Start the VM with: sudo virsh start %s\n", vmName)
}

func displayHelp() {
	fmt.Println("kvm-vm-persistent-net - Set persistent network interface names for KVM VMs")
	fmt.Println("\nUsage:")
	fmt.Println("  kvm-vm-persistent-net [flags] <vm-name>")
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  kvm-vm-persistent-net centos7-vm")
	fmt.Println("  kvm-vm-persistent-net --prefix enp centos7-vm")
	fmt.Println("  kvm-vm-persistent-net --start-index 1 ubuntu-vm")
	fmt.Println("  kvm-vm-persistent-net --dry-run debian-vm")
}

func checkVMStatus(vmName string) error {
	// Check if VM exists and is shut off
	cmd := exec.Command("sudo", "virsh", "list", "--state-shutoff", "--name")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to execute virsh command: %v", err)
	}

	// Check if VM is in the list of shut-off VMs
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == vmName {
			return nil
		}
	}

	// VM not found in shut-off list, check if it exists at all
	cmd = exec.Command("sudo", "virsh", "list", "--all", "--name")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to execute virsh command: %v", err)
	}

	scanner = bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == vmName {
			return fmt.Errorf("VM '%s' exists but is currently running. Please shut it down first", vmName)
		}
	}

	return fmt.Errorf("VM '%s' does not exist", vmName)
}

func getMacAddresses(vmName string) ([]string, error) {
	// Get VM XML using virsh dumpxml
	cmd := exec.Command("sudo", "virsh", "dumpxml", vmName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to execute virsh dumpxml: %v", err)
	}

	// Parse XML to extract MAC addresses
	var domain Domain
	if err := xml.Unmarshal(output, &domain); err != nil {
		return nil, fmt.Errorf("Failed to parse XML: %v", err)
	}

	// Extract MAC addresses
	var macAddresses []string
	for _, iface := range domain.Interfaces {
		if iface.Mac.Address != "" {
			macAddresses = append(macAddresses, iface.Mac.Address)
		}
	}

	return macAddresses, nil
}

func generateRulesFile(macAddresses []string, vmName, prefix string, startIndex int, ruleName string) (string, error) {
	// Create rules file in the current directory
	filePath := ruleName
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("Failed to create rules file: %v", err)
	}
	defer file.Close()

	// Write rules to file
	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "# Network interface persistence rules for VM '%s'\n", vmName)

	for i, mac := range macAddresses {
		interfaceName := fmt.Sprintf("%s%d", prefix, startIndex+i)
		fmt.Fprintf(writer, "SUBSYSTEM==\"net\", ACTION==\"add\", ATTR{address}==\"%s\", NAME=\"%s\"\n", mac, interfaceName)
	}

	writer.Flush()
	return filePath, nil
}

func copyRulesToVM(rulesFile, vmName, ruleName string) error {
	// Use virt-copy-in to copy the file to the VM
	cmd := exec.Command("sudo", "virt-copy-in", "-d", vmName, rulesFile, "/etc/udev/rules.d/")

	// Capture both stdout and stderr
	var stdoutErr bytes.Buffer
	cmd.Stdout = &stdoutErr
	cmd.Stderr = &stdoutErr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to copy rules file to VM: %v\nOutput: %s", err, stdoutErr.String())
	}

	return nil
}
