package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	k3sConfigFile = "/etc/systemd/system/k3s.service"
	nodePortRange = "1000-32000"
	tcpdumpFilter = "udp port 4729 or udp port 9996 or udp port 6343 or udp port 4739"
	captureFile   = "capture.pcap"
)
const scriptContent = `#!/bin/bash

K3S_CONFIG_FILE="/etc/systemd/system/k3s.service"
NODEPORT_RANGE="1000-32000"

update_nodeport_range() {
  echo "Updating K3s NodePort range to ${NODEPORT_RANGE}..."
  cp "${K3S_CONFIG_FILE}" "${K3S_CONFIG_FILE}.bak"
  if [ $? -ne 0 ]; then
    echo "Failed to back up the K3s service file. Exiting."
    exit 1
  fi
  sed -i "s|^ExecStart=.*|ExecStart=/usr/local/bin/k3s server --service-node-port-range=${NODEPORT_RANGE}|" "${K3S_CONFIG_FILE}"
  if [ $? -ne 0 ]; then
    echo "Failed to update the K3s service file. Exiting."
    exit 1
  fi
}

restart_k3s() {
  echo "Restarting K3s service to apply changes..."
  systemctl daemon-reload
  systemctl restart k3s

  if [ $? -ne 0 ]; then
    echo "Failed to restart K3s service. Exiting."
    exit 1
  fi

  echo "K3s service restarted successfully."
}

update_nodeport_range
restart_k3s

echo "NodePort range updated to ${NODEPORT_RANGE} and K3s restarted successfully."
`

// Configuration struct to hold all configurable parameters
type Config struct {
	PodName            string
	ContainerName      string
	ServiceName        string
	DependentPods      []string
	K3sConfigFile      string
	NodePortRange      string
	TcpdumpFilter      string
	CaptureFile        string
	LogFile            string
	VerboseConfigPath  string
	VerboseConfigValue string
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

type Pod struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type Service struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

var config Config

func init() {
	// Define command line flags
	flag.StringVar(&config.PodName, "pod", "", "Name of the main pod to monitor")
	flag.StringVar(&config.ContainerName, "container", "", "Name of the container within the pod")
	flag.StringVar(&config.ServiceName, "service", "", "Name of the service to monitor")
	dependentPodsStr := flag.String("dependent-pods", "", "Comma-separated list of dependent pods")
	flag.StringVar(&config.K3sConfigFile, "k3s-config", "/etc/systemd/system/k3s.service", "Path to K3s config file")
	flag.StringVar(&config.NodePortRange, "nodeport-range", "1000-32000", "NodePort range")
	flag.StringVar(&config.TcpdumpFilter, "tcpdump-filter", "udp", "tcpdump filter string")
	flag.StringVar(&config.CaptureFile, "capture-file", "packets.pcap", "Packet capture file name")
	flag.StringVar(&config.LogFile, "log-file", "debug.log", "Log file name")
	flag.StringVar(&config.VerboseConfigPath, "verbose-config-path", "/etc/config/config.conf", "Path to verbose config file")
	flag.StringVar(&config.VerboseConfigValue, "verbose-config-value", "verbose: enabled", "Value to add to verbose config")

	// Parse flags
	flag.Parse()

	// Process dependent pods
	if *dependentPodsStr != "" {
		config.DependentPods = strings.Split(*dependentPodsStr, ",")
	}

	// Validate required flags
	if config.PodName == "" || config.ContainerName == "" || config.ServiceName == "" {
		fmt.Println("Error: Required flags -pod, -container, and -service must be provided")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func printProgress(current, total int, prefix string) {
	width := 40
	percentage := float64(current) * 100 / float64(total)
	completed := int(float64(width) * float64(current) / float64(total))
	remaining := width - completed

	fmt.Printf("\r%s [%s%s] %.1f%% ", prefix,
		strings.Repeat("=", completed),
		strings.Repeat(" ", remaining),
		percentage)

	if current == total {
		fmt.Println()
	}
}

func printSpinner(duration time.Duration, message string) {
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	startTime := time.Now()

	for time.Since(startTime) < duration {
		for _, char := range spinChars {
			fmt.Printf("\r%s %s", char, message)
			time.Sleep(100 * time.Millisecond)
		}
	}
	fmt.Println()
}

func showMenu() string {
	fmt.Printf("\n%sNetwork Monitoring Debug Tool - Available Options%s\n", colorCyan, colorReset)
	fmt.Println("------------------------------------------------")
	fmt.Println("1. Check pod and service status")
	fmt.Println("2. Update node port range and restart k3s")
	fmt.Println("3. View network packets source IP addresses")
	fmt.Println("4. Capture network packets to file")
	fmt.Println("5. Collect debug logs")
	fmt.Println("6. Exit")
	fmt.Printf("\n%sEnter your choice (1-6):%s ", colorYellow, colorReset)

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	return strings.TrimSpace(choice)
}
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
func updateNodePortRange() {
	fmt.Printf("Updating K3s NodePort range to %s...\n", nodePortRange)

	backupFile := k3sConfigFile + ".bak"
	err := copyFile(k3sConfigFile, backupFile)
	if err != nil {
		fmt.Println("Failed to back up the K3s service file. Exiting.")
	}
	tmpFile, err := ioutil.TempFile("", "update_k3s_nodeport_*.sh")
	if err != nil {
		fmt.Println("Error creating temp file:", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(scriptContent)); err != nil {
		fmt.Println("Error writing to temp file:", err)
		os.Exit(1)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		fmt.Println("Error making script executable:", err)
		os.Exit(1)
	}

	cmd := exec.Command("/bin/bash", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Running the script...")
	if err := cmd.Run(); err != nil {
		fmt.Println("Error executing script:", err)
		os.Exit(1)
	}

	fmt.Println("Script executed successfully.")

	fmt.Println("K3s service file updated successfully.")
}

func collectLogs() bool {
	fmt.Printf("%sEnabling debug logs in pod %s...%s\n", colorCyan, config.PodName, colorReset)

	verboseCmd := fmt.Sprintf("kubectl exec -it $(kubectl get pod -l app=%s -o jsonpath='{.items[0].metadata.name}') -c %s -- sh -c \"echo '%s' >> %s\"",
		config.PodName, config.ContainerName, config.VerboseConfigValue, config.VerboseConfigPath)

	cmd := exec.Command("sh", "-c", verboseCmd)

	if err := cmd.Run(); err != nil {
		fmt.Printf("%sError: Failed to enable debug logs: %v%s\n", colorRed, err, colorReset)
		return false
	}

	fmt.Printf("%sStarting log collection for 5 minutes...%s\n", colorGreen, colorReset)
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Minute)

	file, err := os.Create(config.LogFile)
	if err != nil {
		fmt.Printf("%sError: Failed to create log file: %v%s\n", colorRed, err, colorReset)
		return false
	}
	defer file.Close()

	cmd = exec.Command("kubectl", "logs", "-f", getPodName(config.PodName), "-c", config.ContainerName)
	cmd.Stdout = file

	if err := cmd.Start(); err != nil {
		fmt.Printf("%sError: Failed to start log collection: %v%s\n", colorRed, err, colorReset)
		return false
	}

	for time.Now().Before(endTime) {
		elapsed := time.Since(startTime)
		progress := int(elapsed.Seconds() * 100 / 300)
		printProgress(progress, 100, "Collecting logs: ")
		time.Sleep(1 * time.Second)
	}

	cmd.Process.Kill()
	return true
}

func getPodName(prefix string) string {
	cmd := exec.Command("kubectl", "get", "pods", "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	podNames := strings.Fields(string(output))
	for _, name := range podNames {
		if strings.HasPrefix(name, prefix) {
			return name
		}
	}
	return ""
}

func checkPod(podName string) {
	out, err := exec.Command("kubectl", "get", "pods", "-o", "json").Output()
	if err != nil {
		fmt.Printf("%sError getting pods: %v%s\n", colorRed, err, colorReset)
		return
	}

	var podList struct {
		Items []Pod `json:"items"`
	}
	json.Unmarshal(out, &podList)

	for _, pod := range podList.Items {
		if strings.Contains(pod.Metadata.Name, podName) {
			fmt.Printf("%sPod %s is in status: %s%s\n", colorGreen, podName, pod.Status.Phase, colorReset)
			return
		}
	}
	fmt.Printf("%sPod %s not found!%s\n", colorYellow, podName, colorReset)
}

func checkService(serviceName string) {
	out, err := exec.Command("kubectl", "get", "services", "-o", "json").Output()
	if err != nil {
		fmt.Printf("%sError getting services: %v%s\n", colorRed, err, colorReset)
		return
	}

	var serviceList struct {
		Items []Service `json:"items"`
	}
	json.Unmarshal(out, &serviceList)

	for _, service := range serviceList.Items {
		if service.Metadata.Name == serviceName {
			fmt.Printf("%sService %s is running%s\n", colorGreen, serviceName, colorReset)
			return
		}
	}
	fmt.Printf("%sService %s not found!%s\n", colorYellow, serviceName, colorReset)
}

func capturePacketsForOneMinute() {
	fmt.Printf("%sStarting packet capture for 1 minute...%s\n", colorCyan, colorReset)
	cmd := exec.Command("tcpdump", "-i", "any", "-nn", config.TcpdumpFilter, "-w", config.CaptureFile)

	if err := cmd.Start(); err != nil {
		fmt.Printf("%sError starting tcpdump: %v%s\n", colorRed, err, colorReset)
		return
	}

	startTime := time.Now()
	endTime := startTime.Add(1 * time.Minute)

	for time.Now().Before(endTime) {
		elapsed := time.Since(startTime)
		progress := int(elapsed.Seconds() * 100 / 60)
		printProgress(progress, 100, "Capturing packets: ")
		time.Sleep(1 * time.Second)
	}

	cmd.Process.Kill()
	fmt.Printf("%sPacket capture completed and saved to %s%s\n", colorGreen, config.CaptureFile, colorReset)
}

func collectUniqueIPs() map[string]bool {
	cmd := exec.Command("tcpdump", "-i", "any", "-nn", config.TcpdumpFilter)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("%sError creating stdout pipe: %v%s\n", colorRed, err, colorReset)
		return nil
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("%sError starting tcpdump: %v%s\n", colorRed, err, colorReset)
		return nil
	}

	defer cmd.Process.Kill()

	ipRegex := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	uniqueIPs := make(map[string]bool)
	scanner := bufio.NewScanner(stdout)

	go func() {
		time.Sleep(10 * time.Second)
		cmd.Process.Kill()
	}()

	for scanner.Scan() {
		line := scanner.Text()
		matches := ipRegex.FindAllString(line, -1)
		for _, ip := range matches {
			uniqueIPs[ip] = true
		}
	}

	return uniqueIPs
}

func main() {
	fmt.Printf("\n%sNetwork Monitoring Debug Tool v1.0%s\n", colorCyan, colorReset)
	fmt.Printf("Monitoring pod: %s, container: %s, service: %s\n",
		config.PodName, config.ContainerName, config.ServiceName)
	fmt.Println("This tool helps you troubleshoot network monitoring and packet collection issues")

	for {
		choice := showMenu()

		switch choice {
		case "1":
			checkPod(config.PodName)
			checkService(config.ServiceName)
			for _, pod := range config.DependentPods {
				checkPod(pod)
			}
		case "2":
			updateNodePortRange()
		case "3":
			fmt.Printf("%sCollecting unique IPs (10 second sample)...%s\n", colorCyan, colorReset)
			printSpinner(10*time.Second, "Analyzing network traffic")
			uniqueIPs := collectUniqueIPs()
			if len(uniqueIPs) > 0 {
				fmt.Printf("\n%sDiscovered IPs:%s\n", colorGreen, colorReset)
				for ip := range uniqueIPs {
					fmt.Printf("  - %s\n", ip)
				}
			} else {
				fmt.Println("No packets received during sampling period")
			}
		case "4":
			capturePacketsForOneMinute()
		case "5":
			if collectLogs() {
				fmt.Printf("%sLogs collected successfully. Please check %s%s\n",
					colorGreen, config.LogFile, colorReset)
			}
		case "6":
			fmt.Printf("\n%sThank you for using Network Monitoring Debug Tool. Goodbye!%s\n",
				colorCyan, colorReset)
			return
		default:
			fmt.Printf("%sInvalid choice. Please select a number between 1 and 6.%s\n",
				colorYellow, colorReset)
		}

		fmt.Printf("\nPress Enter to continue...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}
