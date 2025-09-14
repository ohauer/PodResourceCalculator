# Pod Resource Calculator

A Kubernetes resource spreadsheet calculator tool that generates Excel reports of pod resource requests and limits across your cluster.

## Features

- **Kubernetes Integration**: Connects to any Kubernetes cluster (in-cluster or via kubeconfig)
- **Resource Analysis**: Extracts CPU and memory requests/limits for all containers
- **Excel Output**: Generates professional Excel spreadsheets with formulas and formatting
- **Namespace Filtering**: Support for specific namespaces or cluster-wide analysis
- **Multi-platform**: Builds for Linux, Windows, macOS, and FreeBSD
- **Modern Go**: Built with Go 1.23 and latest Kubernetes client libraries

## Prerequisites

- Access to a Kubernetes cluster
- Valid kubeconfig file (for out-of-cluster usage)
- Or running inside a Kubernetes cluster with appropriate RBAC permissions

## Quick Start

### Build and Run
```bash
cd src
make build
../PodResourceCalculator -verbose
```

### Usage Examples
```bash
# Analyze all namespaces
./PodResourceCalculator -verbose

# Analyze specific namespace
./PodResourceCalculator -namespace kube-system -verbose

# Custom output filename
./PodResourceCalculator -output my-resources.xlsx

# Use specific kubeconfig
./PodResourceCalculator -kubeconfig ~/.kube/config-prod
```

## Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-namespace` | Kubernetes namespace to analyze | All namespaces |
| `-kubeconfig` | Path to kubeconfig file | `~/.kube/config` |
| `-output` | Output Excel filename | `resource_YYYY-MM-DD.xlsx` |
| `-verbose` | Enable verbose logging | `false` |

## Excel Output

The generated Excel file contains four comprehensive sheets:

### Resources Sheet (Detailed Container Data)
- **Namespace**: Pod namespace
- **Pod**: Pod name
- **Node**: Host node IP
- **Container**: Container name
- **Status**: Pod status (Running, Pending, etc.)
- **Request CPU (m)**: CPU requests in millicores
- **Request CPU**: CPU requests (canonical format)
- **Request Memory (Mi)**: Memory requests in mebibytes (1 decimal)
- **Request Memory**: Memory requests (canonical format)
- **Limit CPU (m)**: CPU limits in millicores
- **Limit CPU**: CPU limits (canonical format)
- **Limit Memory (Mi)**: Memory limits in mebibytes (1 decimal)
- **Limit Memory**: Memory limits (canonical format)
- **CPU Efficiency %**: Request/Limit ratio for CPU
- **Memory Efficiency %**: Request/Limit ratio for Memory

### Summary Sheet (Namespace Aggregation)
- **Namespace-level totals**: Resource aggregation per namespace
- **CPU in cores**: Request and limit CPU converted to cores
- **Memory in Mi**: Request and limit memory converted to mebibytes (Mi)
- **Alphabetical sorting**: Namespaces sorted for easy navigation
- **Clean data table**: Optimized for analysis and reference

### Nodes Sheet (Node Utilization)
- **Node IP**: Host node identifier
- **Pod Count**: Number of pods per node
- **Resource totals**: CPU and memory requests/limits per node
- **Capacity planning**: Understand node resource distribution
- **Alphabetical sorting**: Nodes sorted by IP address

### Chart Sheet (Visual Analytics)
- **Dynamic bar chart**: Resource requirements by namespace
- **Scalable dimensions**: Chart size adapts to data volume (1.5x scaling)
- **Top legend**: Professional layout with legend at top
- **Four data series**: Request CPU, Limit CPU, Request Memory, Limit Memory
- **Cross-sheet references**: Automatically updates with data changes

### Features
- **Auto-filter**: Easy sorting and filtering on Resources sheet
- **Conditional formatting**: Color-coded efficiency percentages
  - Red (≥80%): High resource utilization
  - Yellow (60-79%): Medium utilization
  - Teal (40-59%): Low utilization
  - Light Green (<40%): Very low utilization
- **Progress indicators**: Shows processing progress for large clusters
- **Pod status filtering**: Only includes Running and Pending pods
- **Missing resource handling**: Shows "Not Set" for containers without limits/requests
- **Summary formulas**: Automatic totals in Resources sheet row 1
  - F1: Total CPU requests (cores)
  - H1: Total memory requests (Mi)
  - J1: Total CPU limits (cores)
  - L1: Total memory limits (Mi)
- **Freeze panes**: Header rows stay visible when scrolling
- **Optimized column widths**: Properly sized for content readability
- **Professional charts**: Dedicated chart sheet with dynamic sizing
- **Alphabetical sorting**: Consistent ordering across all sheets
- **Multi-dimensional analysis**: Container, namespace, and node-level views

## Build System

### Available Make Targets
```bash
make help          # Show all available targets
make build         # Build for current platform
make build-all     # Build for all platforms
make test          # Run tests
make clean         # Clean build artifacts
make deps          # Download and tidy dependencies
make run           # Build and run with verbose output
```

### Multi-platform Builds
```bash
make build-all
```
Generates binaries for:
- Linux (amd64)
- Windows (amd64)
- macOS (amd64)
- FreeBSD (amd64)

## Development

### Project Structure
```
PodResourceCalculator/
├── src/
│   ├── main.go           # Main application
│   ├── Makefile          # Build automation
│   ├── go.mod            # Go module definition
│   └── go.sum            # Dependency checksums
├── .vscode/
│   └── settings.json     # VSCode configuration
├── .editorconfig         # Editor configuration
├── .gitignore           # Git ignore rules
└── README.md            # This file
```

### Dependencies
- **Kubernetes Client**: `k8s.io/client-go v0.31.2`
- **Excel Generation**: `github.com/xuri/excelize/v2 v2.9.1`
- **Logging**: `github.com/sirupsen/logrus v1.9.3`

## RBAC Requirements

For in-cluster usage, ensure the service account has appropriate permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pod-resource-reader
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pod-resource-reader-binding
subjects:
- kind: ServiceAccount
  name: your-service-account
  namespace: your-namespace
roleRef:
  kind: ClusterRole
  name: pod-resource-reader
  apiGroup: rbac.authorization.k8s.io
```

## Examples

### Basic Usage
```bash
# Generate report for all namespaces
./PodResourceCalculator

# Generate report with custom filename
./PodResourceCalculator -output cluster-resources-$(date +%Y%m%d).xlsx
```

### Integration with kubectl
```bash
# Check if you have access to pods first
kubectl auth can-i list pods --all-namespaces

# Run the calculator
./PodResourceCalculator -verbose
```

### Docker Usage
```bash
# Build in container
docker run --rm -v $(PWD):/workspace -w /workspace/src golang:1.23-alpine \
  sh -c "go build -o ../PodResourceCalculator ."

# Run with mounted kubeconfig
docker run --rm -v $(PWD):/workspace -v ~/.kube:/root/.kube \
  -w /workspace your-image ./PodResourceCalculator
```

## Troubleshooting

### Common Issues

**"Failed to connect to Kubernetes"**
- Verify kubeconfig is valid: `kubectl cluster-info`
- Check network connectivity to cluster
- Ensure proper RBAC permissions

**"Failed to list pods"**
- Check RBAC permissions for pod listing
- Verify namespace exists (if specified)
- Try with `-verbose` flag for detailed logging

**Empty Excel file**
- No pods found in specified namespace
- All pods may lack resource specifications
- Check with `kubectl get pods -n <namespace>`

## License

This project is open source. See the original blog post for more details:
https://medium.com/@zhimin.wen/pod-resource-spreadsheet-calculator-22fc5c6173b9

Since pull requests were never answered, this repo is no longer linked to the original or any other fork
