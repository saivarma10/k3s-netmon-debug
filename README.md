# k8s-netmon-debug

A command-line debugging tool for Kubernetes network monitoring applications. This tool helps troubleshoot network monitoring pods, manage K3s configurations, and capture network traffic for analysis.

## Features

- 🔍 Pod and service health checking
- 🔄 K3s node port range configuration
- 📊 Network packet analysis
- 📥 Packet capture capabilities
- 📝 Debug log collection
- 🎨 Interactive CLI with progress indicators

## Prerequisites

- Go 1.19 or later
- Kubernetes/K3s cluster with admin access
- tcpdump installed on the host
- kubectl configured with appropriate permissions

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/k8s-netmon-debug.git
cd k8s-netmon-debug

# Build the binary
go build -o k8s-netmon-debug main.go
```

## Usage

Basic usage:

```bash
./k8s-netmon-debug \
  -pod="npm-collector" \
  -container="npm-collector-app" \
  -service="npm-collector" \
  -dependent-pods="stan-0" \
  -tcpdump-filter="udp port 4729 or udp port 9996" \
  -log-file="debug.log"
```

### Available Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-pod` | Name of the main pod to monitor | Required |
| `-container` | Name of the container within the pod | Required |
| `-service` | Name of the service to monitor | Required |
| `-dependent-pods` | Comma-separated list of dependent pods | "" |
| `-k3s-config` | Path to K3s config file | "/etc/systemd/system/k3s.service" |
| `-nodeport-range` | NodePort range for K3s | "1000-32000" |
| `-tcpdump-filter` | tcpdump filter string | "udp" |
| `-capture-file` | Packet capture file name | "packets.pcap" |
| `-log-file` | Log file name | "debug.log" |

## Features in Detail

### 1. Pod and Service Status
Checks the status of specified pods and services in your Kubernetes cluster.

### 2. K3s NodePort Management
Updates the NodePort range in K3s configuration and handles service restart.

### 3. Network Traffic Analysis
Captures and analyzes network traffic using tcpdump with customizable filters.

### 4. Debug Log Collection
Collects detailed logs from specified containers with progress tracking.

### 5. Packet Capture
Captures network packets to a file for detailed analysis.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch 
3. Commit your changes 
4. Push to the branch 
5. Open a Pull Request
