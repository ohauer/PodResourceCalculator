package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Constants for resource processing and efficiency thresholds
const (
	// Processing batch sizes
	ProcessingBatchSize = 50  // Log progress every N pods
	MemoryLogInterval   = 500 // Log memory usage every N pods

	// Efficiency thresholds (percentage)
	HighEfficiency   = 80 // Red - high resource utilization
	MediumEfficiency = 60 // Yellow - medium utilization
	LowEfficiency    = 40 // Teal - low utilization
	// Below LowEfficiency = Light green - very low utilization

	// Over/under provisioning thresholds
	OverProvisionedThreshold  = 50 // Below this = over-provisioned
	UnderProvisionedThreshold = 80 // Above this = under-provisioned

	// API timeout
	DefaultAPITimeout = 30 * time.Second

	// Chart dimensions
	ChartBaseWidth   = 800
	ChartBaseHeight  = 600
	ChartWidthScale  = 2.5
	ChartHeightScale = 3
	ChartRowHeight   = 60
	ChartMaxHeight   = 3600
)

// validatePath checks if a file path is safe from path traversal attacks
func validatePath(path string) error {
	if path == "" {
		return nil
	}
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	dangerousPrefixes := []string{"/etc/", "/sys/", "/proc/", "/dev/"}
	for _, prefix := range dangerousPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return fmt.Errorf("access to system directories not allowed: %s", path)
		}
	}
	return nil
}

// validateNamespace checks if a namespace name is valid
func validateNamespace(namespace string) error {
	if namespace == "" {
		return nil // Empty namespace means all namespaces
	}
	if len(namespace) > 63 {
		return fmt.Errorf("namespace too long (max 63 characters)")
	}
	validNamespace := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validNamespace.MatchString(namespace) {
		return fmt.Errorf("invalid namespace format: %s", namespace)
	}
	return nil
}

// setCellStyle sets cell style and logs error if it fails
func setCellStyle(f *excelize.File, sheet, hCell, vCell string, styleID int) {
	if err := f.SetCellStyle(sheet, hCell, vCell, styleID); err != nil {
		logrus.Warnf("Failed to set cell style for %s:%s-%s: %v", sheet, hCell, vCell, err)
	}
}

func main() {
	var (
		namespace  = flag.String("namespace", os.Getenv("K8S_NAMESPACE"), "Kubernetes namespace (default: all namespaces)")
		kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config)")
		output     = flag.String("output", "", "Output filename (default: resource_YYYY-MM-DD.xlsx)")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Validate namespace
	if *namespace != "" {
		if err := validateNamespace(*namespace); err != nil {
			logrus.Fatalf("Invalid namespace: %v", err)
		}
	}

	// Validate kubeconfig path
	if *kubeconfig != "" {
		if err := validatePath(*kubeconfig); err != nil {
			logrus.Fatalf("Invalid kubeconfig path: %v", err)
		}
	}

	// Validate output filename
	filename := getOutputFilename(*output)
	if err := validatePath(filename); err != nil {
		logrus.Fatalf("Invalid output filename: %v", err)
	}

	clientSet, err := getK8sClient(*kubeconfig)
	if err != nil {
		logrus.Fatalf("Failed to connect to Kubernetes: %v", err)
	}

	logrus.Infof("Fetching pods from namespace: %s", getNamespaceDisplay(*namespace))
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	pods, err := clientSet.CoreV1().Pods(*namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Fatalf("Failed to list pods: %v", err)
	}

	logrus.Infof("Found %d pods", len(pods.Items))

	if err := generateExcel(pods.Items, filename); err != nil {
		logrus.Fatalf("Failed to generate Excel file: %v", err)
	}

	logrus.Infof("Excel file created: %s", filename)
}

func getK8sClient(kubeconfigPath string) (kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	// Check if running inside cluster
	if _, inCluster := os.LookupEnv("KUBERNETES_SERVICE_HOST"); inCluster {
		logrus.Debug("Using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		logrus.Debug("Using kubeconfig file")
		if kubeconfigPath == "" {
			if home := homeDir(); home != "" {
				kubeconfigPath = filepath.Join(home, ".kube", "config")
			}
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientSet, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}

func getNamespaceDisplay(namespace string) string {
	if namespace == "" {
		return "all namespaces"
	}
	return namespace
}

func getOutputFilename(output string) string {
	if output != "" {
		return filepath.Clean(output)
	}
	return fmt.Sprintf("resource_%s.xlsx", time.Now().Format("2006-01-02"))
}

func generateExcel(pods []corev1.Pod, filename string) error {
	f := excelize.NewFile()
	defer f.Close()

	// Define sheet names
	sheet1Name, sheet2Name, sheet3Name, sheet4Name, sheet5Name := "Resources", "Namespaces", "Nodes", "Chart", "Insights"

	index, err := f.NewSheet(sheet1Name)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Delete default Sheet1
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return fmt.Errorf("failed to delete default sheet: %w", err)
	}

	// Set headers
	headers := []string{
		"Namespace", "Pod", "Node", "Container", "Status",
		"Request CPU (m)", "Request CPU", "Request Memory (Mi)", "Request Memory",
		"Limit CPU (m)", "Limit CPU", "Limit Memory (Mi)", "Limit Memory",
		"CPU Efficiency %", "Memory Efficiency %", "CPU % of Cluster", "Memory % of Cluster",
	}

	if err := f.SetSheetRow(sheet1Name, "A2", &headers); err != nil {
		return fmt.Errorf("failed to set headers: %w", err)
	}

	// Set auto filter
	if err := f.AutoFilter(sheet1Name, "A2:Q2", []excelize.AutoFilterOptions{}); err != nil {
		return fmt.Errorf("failed to set auto filter: %w", err)
	}

	// Single-pass data processing with aggregation
	logrus.Infof("Processing %d pods...", len(pods))
	logMemoryUsage("start processing")

	// Pre-calculate cluster totals for percentage calculations
	var clusterTotalReqCPU, clusterTotalReqMem int64
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}
		for _, container := range pod.Spec.Containers {
			if reqCPU := container.Resources.Requests.Cpu(); reqCPU != nil {
				clusterTotalReqCPU += reqCPU.MilliValue()
			}
			if reqMem := container.Resources.Requests.Memory(); reqMem != nil {
				clusterTotalReqMem += reqMem.Value()
			}
		}
	}

	// Data structures for aggregation
	namespaceTotals := make(map[string]struct {
		reqCPU, limCPU int64
		reqMem, limMem int64
	})
	nodeTotals := make(map[string]struct {
		podCount       int
		reqCPU, limCPU int64
		reqMem, limMem int64
	})

	row := 3
	processedContainers := 0
	for i, pod := range pods {
		if i%50 == 0 && i > 0 {
			logrus.Infof("Processed %d/%d pods (%d containers)", i, len(pods), processedContainers)
			if i%500 == 0 {
				logMemoryUsage(fmt.Sprintf("after %d pods", i))
			}
		}

		// Filter by pod status
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		// Track pod count per node
		node := pod.Status.HostIP
		if node == "" {
			node = "Unknown"
		}
		nodeTotal := nodeTotals[node]
		nodeTotal.podCount++

		for _, container := range pod.Spec.Containers {
			reqCPU := container.Resources.Requests.Cpu()
			reqMem := container.Resources.Requests.Memory()
			limCPU := container.Resources.Limits.Cpu()
			limMem := container.Resources.Limits.Memory()

			// Better missing resource handling
			reqCPUVal := int64(0)
			reqCPUStr := "-"
			if reqCPU != nil && !reqCPU.IsZero() {
				reqCPUVal = reqCPU.MilliValue()
				reqCPUStr = reqCPU.String()
			}

			reqMemVal := float64(0)
			reqMemStr := "-"
			if reqMem != nil && !reqMem.IsZero() {
				reqMemVal = float64(reqMem.Value()) / (1024 * 1024)
				reqMemStr = reqMem.String()
			}

			limCPUVal := int64(0)
			limCPUStr := "-"
			if limCPU != nil && !limCPU.IsZero() {
				limCPUVal = limCPU.MilliValue()
				limCPUStr = limCPU.String()
			}

			limMemVal := float64(0)
			limMemStr := "-"
			if limMem != nil && !limMem.IsZero() {
				limMemVal = float64(limMem.Value()) / (1024 * 1024)
				limMemStr = limMem.String()
			}

			// Calculate efficiency percentages
			cpuEfficiency := ""
			memEfficiency := ""
			if limCPUVal > 0 && reqCPUVal > 0 {
				cpuEfficiency = fmt.Sprintf("%.1f%%", float64(reqCPUVal)/float64(limCPUVal)*100)
			}
			if limMemVal > 0 && reqMemVal > 0 {
				memEfficiency = fmt.Sprintf("%.1f%%", reqMemVal/limMemVal*100)
			}

			// Aggregate data for other sheets
			ns := pod.Namespace
			if ns == "" {
				ns = "default"
			}
			nsTotals := namespaceTotals[ns]
			nsTotals.reqCPU += reqCPUVal
			nsTotals.limCPU += limCPUVal
			if reqMem != nil {
				nsTotals.reqMem += reqMem.Value()
			}
			if limMem != nil {
				nsTotals.limMem += limMem.Value()
			}
			namespaceTotals[ns] = nsTotals

			// Update node totals (accumulated for all containers in this pod)
			nodeTotal.reqCPU += reqCPUVal
			nodeTotal.limCPU += limCPUVal
			if reqMem != nil {
				nodeTotal.reqMem += reqMem.Value()
			}
			if limMem != nil {
				nodeTotal.limMem += limMem.Value()
			}

			// Calculate cluster percentages
			cpuClusterPct := ""
			memClusterPct := ""
			if clusterTotalReqCPU > 0 {
				cpuClusterPct = fmt.Sprintf("%.2f%%", float64(reqCPUVal)/float64(clusterTotalReqCPU)*100)
			}
			if clusterTotalReqMem > 0 && reqMem != nil {
				memClusterPct = fmt.Sprintf("%.2f%%", float64(reqMem.Value())/float64(clusterTotalReqMem)*100)
			}

			rowData := []interface{}{
				pod.Namespace,
				pod.Name,
				pod.Status.HostIP,
				container.Name,
				string(pod.Status.Phase),
				reqCPUVal, reqCPUStr,
				reqMemVal, reqMemStr,
				limCPUVal, limCPUStr,
				limMemVal, limMemStr,
				cpuEfficiency,
				memEfficiency,
				cpuClusterPct,
				memClusterPct,
			}

			// Write to Resources sheet with enhanced error context
			context := fmt.Sprintf("pod '%s' container '%s'", pod.Name, container.Name)
			if err := setRowWithContext(f, sheet1Name, row, rowData, context); err != nil {
				return err
			}

			// Format memory columns to 1 decimal place
			hCell, _ := excelize.CoordinatesToCellName(8, row)  // Column H (Request Memory Mi)
			lCell, _ := excelize.CoordinatesToCellName(12, row) // Column L (Limit Memory Mi)
			f.SetCellStyle(sheet1Name, hCell, hCell, getNumberStyle(f))
			f.SetCellStyle(sheet1Name, lCell, lCell, getNumberStyle(f))

			// Apply conditional formatting for efficiency
			nCell, _ := excelize.CoordinatesToCellName(14, row) // CPU Efficiency
			oCell, _ := excelize.CoordinatesToCellName(15, row) // Memory Efficiency
			if cpuEfficiency != "" {
				f.SetCellStyle(sheet1Name, nCell, nCell, getEfficiencyStyle(f, cpuEfficiency))
			}
			if memEfficiency != "" {
				f.SetCellStyle(sheet1Name, oCell, oCell, getEfficiencyStyle(f, memEfficiency))
			}

			row++
			processedContainers++
		}
		
		// Update node totals once after processing all containers in the pod
		nodeTotals[node] = nodeTotal
	}

	logrus.Infof("Completed processing: %d pods, %d containers", len(pods), processedContainers)
	logMemoryUsage("after processing")

	// Data validation and warnings
	validateAndWarnResources(namespaceTotals, nodeTotals, processedContainers)

	// Add summary formulas
	if err := addSummaryFormulas(f, sheet1Name, row); err != nil {
		return fmt.Errorf("failed to add summary formulas: %w", err)
	}

	// Set column widths for better readability
	if err := setColumnWidths(f, sheet1Name); err != nil {
		return fmt.Errorf("failed to set column widths: %w", err)
	}

	// Create summary sheet with charts
	if err := createSummarySheetFromData(f, namespaceTotals, sheet2Name); err != nil {
		return fmt.Errorf("failed to create summary sheet: %w", err)
	}

	// Create node utilization sheet
	if err := createNodeSheetFromData(f, nodeTotals, sheet3Name); err != nil {
		return fmt.Errorf("failed to create node sheet: %w", err)
	}

	// Create dedicated chart sheet
	if err := createChartSheetFromData(f, namespaceTotals, sheet4Name, sheet2Name); err != nil {
		return fmt.Errorf("failed to create chart sheet: %w", err)
	}

	// Create data science insights sheet
	if err := createInsightsSheet(f, namespaceTotals, nodeTotals, processedContainers, sheet5Name); err != nil {
		return fmt.Errorf("failed to create insights sheet: %w", err)
	}

	// Freeze panes
	if err := setPanes(f, sheet1Name); err != nil {
		return fmt.Errorf("failed to set panes: %w", err)
	}

	// Set Resources sheet as active for better UX
	if idx, err := f.GetSheetIndex(sheet1Name); err == nil && idx >= 0 {
		f.SetActiveSheet(idx)
	}

	// Save file
	if err := f.SaveAs(filename); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

func addSummaryFormulas(f *excelize.File, sheetName string, lastRow int) error {
	formulas := map[string]string{
		"F1": fmt.Sprintf("SUBTOTAL(109,F3:F%d)/1000", lastRow-1), // CPU requests in cores
		"H1": fmt.Sprintf("SUBTOTAL(109,H3:H%d)", lastRow-1),      // Memory requests in Mi
		"J1": fmt.Sprintf("SUBTOTAL(109,J3:J%d)/1000", lastRow-1), // CPU limits in cores
		"L1": fmt.Sprintf("SUBTOTAL(109,L3:L%d)", lastRow-1),      // Memory limits in Mi
	}

	for cell, formula := range formulas {
		if err := f.SetCellFormula(sheetName, cell, formula); err != nil {
			return fmt.Errorf("failed to set formula for cell %s: %w", cell, err)
		}
	}

	return nil
}

func setPanes(f *excelize.File, sheetName string) error {
	return f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      2,
		TopLeftCell: "A3",
		ActivePane:  "bottomLeft",
		Selection: []excelize.Selection{
			{SQRef: "A3", ActiveCell: "A3", Pane: "bottomLeft"},
		},
	})
}

func setColumnWidths(f *excelize.File, sheetName string) error {
	// Optimal column widths based on typical content
	columnWidths := map[string]float64{
		"A": 15, // Namespace
		"B": 25, // Pod
		"C": 15, // Node
		"D": 20, // Container
		"E": 10, // Status
		"F": 12, // Request CPU (m)
		"G": 15, // Request CPU
		"H": 18, // Request Memory (Mi)
		"I": 15, // Request Memory
		"J": 12, // Limit CPU (m)
		"K": 15, // Limit CPU
		"L": 18, // Limit Memory (Mi)
		"M": 15, // Limit Memory
		"N": 16, // CPU Efficiency %
		"O": 18, // Memory Efficiency %
		"P": 16, // CPU % of Cluster
		"Q": 18, // Memory % of Cluster
	}

	for col, width := range columnWidths {
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			return fmt.Errorf("failed to set width for column %s: %w", col, err)
		}
	}

	return nil
}

func getNumberStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		NumFmt: 2, // 0.0 format (1 decimal place)
	})
	return style
}

func getEfficiencyStyle(f *excelize.File, efficiency string) int {
	// Extract percentage value
	pctStr := strings.TrimSuffix(efficiency, "%")
	var pct float64
	fmt.Sscanf(pctStr, "%f", &pct)

	// Color based on efficiency
	var fillColor string
	if pct >= 80 {
		fillColor = "FF6B6B" // Red - high usage
	} else if pct >= 60 {
		fillColor = "FFE66D" // Yellow - medium usage
	} else if pct >= 40 {
		fillColor = "4ECDC4" // Teal - low usage
	} else {
		fillColor = "95E1D3" // Light green - very low usage
	}

	style, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{fillColor},
			Pattern: 1,
		},
	})
	return style
}
func createNodeSheet(f *excelize.File, pods []corev1.Pod, sheetName string) error {
	_, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create node sheet: %w", err)
	}

	// Calculate node totals
	nodeTotals := make(map[string]struct {
		podCount       int
		reqCPU, limCPU int64
		reqMem, limMem int64
	})

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		node := pod.Status.HostIP
		if node == "" {
			node = "Unknown"
		}

		totals := nodeTotals[node]
		totals.podCount++

		for _, container := range pod.Spec.Containers {
			if reqCPU := container.Resources.Requests.Cpu(); reqCPU != nil {
				totals.reqCPU += reqCPU.MilliValue()
			}
			if limCPU := container.Resources.Limits.Cpu(); limCPU != nil {
				totals.limCPU += limCPU.MilliValue()
			}
			if reqMem := container.Resources.Requests.Memory(); reqMem != nil {
				totals.reqMem += reqMem.Value()
			}
			if limMem := container.Resources.Limits.Memory(); limMem != nil {
				totals.limMem += limMem.Value()
			}
		}
		nodeTotals[node] = totals
	}

	// Set headers
	headers := []string{"Node IP", "Pod Count", "Request CPU (cores)", "Limit CPU (cores)", "Request Memory (Mi)", "Limit Memory (Mi)"}
	if err := f.SetSheetRow(sheetName, "A1", &headers); err != nil {
		return fmt.Errorf("failed to set headers: %w", err)
	}

	// Sort nodes
	var sortedNodes []string
	for node := range nodeTotals {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Strings(sortedNodes)

	// Set data
	row := 2
	for _, node := range sortedNodes {
		totals := nodeTotals[node]
		data := []interface{}{
			node,
			totals.podCount,
			float64(totals.reqCPU) / 1000,
			float64(totals.limCPU) / 1000,
			float64(totals.reqMem) / (1024 * 1024),
			float64(totals.limMem) / (1024 * 1024),
		}

		cellName, err := excelize.CoordinatesToCellName(1, row)
		if err != nil {
			return fmt.Errorf("failed to get cell name for row %d: %w", row, err)
		}

		if err := f.SetSheetRow(sheetName, cellName, &data); err != nil {
			return fmt.Errorf("failed to set row data: %w", err)
		}

		// Format memory columns
		eCell, _ := excelize.CoordinatesToCellName(5, row)
		fCell, _ := excelize.CoordinatesToCellName(6, row)
		f.SetCellStyle(sheetName, eCell, eCell, getNumberStyle(f))
		f.SetCellStyle(sheetName, fCell, fCell, getNumberStyle(f))

		row++
	}

	// Set column widths
	nodeColumnWidths := map[string]float64{
		"A": 20, // Node IP
		"B": 12, // Pod Count
		"C": 18, // Request CPU
		"D": 16, // Limit CPU
		"E": 20, // Request Memory
		"F": 18, // Limit Memory
	}

	for col, width := range nodeColumnWidths {
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			return fmt.Errorf("failed to set column width: %w", err)
		}
	}

	return nil
}
func createSummarySheetFromData(f *excelize.File, namespaceTotals map[string]struct {
	reqCPU, limCPU int64
	reqMem, limMem int64
}, sheetName string) error {
	_, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create summary sheet: %w", err)
	}

	// Set headers
	headers := []string{"Namespace", "Request CPU (cores)", "Limit CPU (cores)", "Request Memory (Mi)", "Limit Memory (Mi)"}
	if err := f.SetSheetRow(sheetName, "A1", &headers); err != nil {
		return fmt.Errorf("failed to set headers: %w", err)
	}

	// Sort namespaces
	var sortedNamespaces []string
	for ns := range namespaceTotals {
		sortedNamespaces = append(sortedNamespaces, ns)
	}
	sort.Strings(sortedNamespaces)

	// Set data
	row := 2
	var totalReqCPU, totalLimCPU, totalReqMem, totalLimMem int64

	for _, ns := range sortedNamespaces {
		totals := namespaceTotals[ns]
		totalReqCPU += totals.reqCPU
		totalLimCPU += totals.limCPU
		totalReqMem += totals.reqMem
		totalLimMem += totals.limMem

		data := []interface{}{
			ns,
			float64(totals.reqCPU) / 1000,
			float64(totals.limCPU) / 1000,
			float64(totals.reqMem) / (1024 * 1024),
			float64(totals.limMem) / (1024 * 1024),
		}

		if err := setRowWithContext(f, sheetName, row, data, fmt.Sprintf("namespace '%s'", ns)); err != nil {
			return err
		}

		// Format memory columns
		dCell, _ := excelize.CoordinatesToCellName(4, row)
		eCell, _ := excelize.CoordinatesToCellName(5, row)
		f.SetCellStyle(sheetName, dCell, dCell, getNumberStyle(f))
		f.SetCellStyle(sheetName, eCell, eCell, getNumberStyle(f))

		row++
	}

	// Add cluster totals row
	totalData := []interface{}{
		"CLUSTER TOTAL",
		float64(totalReqCPU) / 1000,
		float64(totalLimCPU) / 1000,
		float64(totalReqMem) / (1024 * 1024),
		float64(totalLimMem) / (1024 * 1024),
	}

	if err := setRowWithContext(f, sheetName, row, totalData, "cluster totals"); err != nil {
		return err
	}

	// Format totals row with bold style
	totalStyle := getBoldStyle(f)
	for col := 1; col <= 5; col++ {
		cell, _ := excelize.CoordinatesToCellName(col, row)
		f.SetCellStyle(sheetName, cell, cell, totalStyle)
	}

	// Format memory columns in totals
	dCell, _ := excelize.CoordinatesToCellName(4, row)
	eCell, _ := excelize.CoordinatesToCellName(5, row)
	f.SetCellStyle(sheetName, dCell, dCell, getBoldNumberStyle(f))
	f.SetCellStyle(sheetName, eCell, eCell, getBoldNumberStyle(f))

	// Set column widths
	summaryColumnWidths := map[string]float64{
		"A": 20, "B": 18, "C": 16, "D": 20, "E": 18,
	}

	for col, width := range summaryColumnWidths {
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			return fmt.Errorf("failed to set column width: %w", err)
		}
	}

	return nil
}

func createNodeSheetFromData(f *excelize.File, nodeTotals map[string]struct {
	podCount       int
	reqCPU, limCPU int64
	reqMem, limMem int64
}, sheetName string) error {
	_, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create node sheet: %w", err)
	}

	// Set headers
	headers := []string{"Node IP", "Pod Count", "Request CPU (cores)", "Limit CPU (cores)", "Request Memory (Mi)", "Limit Memory (Mi)"}
	if err := f.SetSheetRow(sheetName, "A1", &headers); err != nil {
		return fmt.Errorf("failed to set headers: %w", err)
	}

	// Sort nodes
	var sortedNodes []string
	for node := range nodeTotals {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Strings(sortedNodes)

	// Set data
	row := 2
	for _, node := range sortedNodes {
		totals := nodeTotals[node]
		data := []interface{}{
			node,
			totals.podCount,
			float64(totals.reqCPU) / 1000,
			float64(totals.limCPU) / 1000,
			float64(totals.reqMem) / (1024 * 1024),
			float64(totals.limMem) / (1024 * 1024),
		}

		cellName, err := excelize.CoordinatesToCellName(1, row)
		if err != nil {
			return fmt.Errorf("failed to get cell name for row %d: %w", row, err)
		}

		if err := f.SetSheetRow(sheetName, cellName, &data); err != nil {
			return fmt.Errorf("failed to set row data: %w", err)
		}

		// Format memory columns
		eCell, _ := excelize.CoordinatesToCellName(5, row)
		fCell, _ := excelize.CoordinatesToCellName(6, row)
		f.SetCellStyle(sheetName, eCell, eCell, getNumberStyle(f))
		f.SetCellStyle(sheetName, fCell, fCell, getNumberStyle(f))

		row++
	}

	// Set column widths
	nodeColumnWidths := map[string]float64{
		"A": 20, "B": 12, "C": 18, "D": 16, "E": 20, "F": 18,
	}

	for col, width := range nodeColumnWidths {
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			return fmt.Errorf("failed to set column width: %w", err)
		}
	}

	return nil
}
func createChartSheetFromData(f *excelize.File, namespaceTotals map[string]struct {
	reqCPU, limCPU int64
	reqMem, limMem int64
}, chartSheetName, summarySheetName string) error {
	if len(namespaceTotals) == 0 {
		return fmt.Errorf("no namespace data available for chart creation")
	}

	// Create regular sheet for chart
	_, err := f.NewSheet(chartSheetName)
	if err != nil {
		return fmt.Errorf("failed to create chart sheet: %w", err)
	}

	lastRow := len(namespaceTotals) + 1

	// Scaled width and height for better readability
	width := uint(800 * 2.5)                                // Factor 2.5 scaling = 2000px
	height := uint((600 + (len(namespaceTotals) * 60)) * 3) // Factor 3 scaling
	if height > 3600 {
		height = 3600
	} // Max height

	// Add CPU chart
	if err := f.AddChart(chartSheetName, "A1", &excelize.Chart{
		Type: excelize.BarStacked,
		Series: []excelize.ChartSeries{
			{
				Name:       fmt.Sprintf("%s!$B$1", summarySheetName), // Request CPU
				Categories: fmt.Sprintf("%s!$A$2:$A$%d", summarySheetName, lastRow),
				Values:     fmt.Sprintf("%s!$B$2:$B$%d", summarySheetName, lastRow),
			},
			{
				Name:       fmt.Sprintf("%s!$C$1", summarySheetName), // Limit CPU
				Categories: fmt.Sprintf("%s!$A$2:$A$%d", summarySheetName, lastRow),
				Values:     fmt.Sprintf("%s!$C$2:$C$%d", summarySheetName, lastRow),
			},
		},
		Title: []excelize.RichTextRun{
			{Text: "CPU Resources by Namespace (cores)"},
		},
		Legend: excelize.ChartLegend{
			Position: "top",
		},
		Dimension: excelize.ChartDimension{
			Width:  width,
			Height: height / 2, // Half height for each chart
		},
	}); err != nil {
		return fmt.Errorf("failed to add CPU chart: %w", err)
	}

	// Add Memory chart below CPU chart
	memoryStartRow := fmt.Sprintf("A%d", int(height/2/15)+5) // Position below CPU chart
	if err := f.AddChart(chartSheetName, memoryStartRow, &excelize.Chart{
		Type: excelize.BarStacked,
		Series: []excelize.ChartSeries{
			{
				Name:       fmt.Sprintf("%s!$D$1", summarySheetName), // Request Memory
				Categories: fmt.Sprintf("%s!$A$2:$A$%d", summarySheetName, lastRow),
				Values:     fmt.Sprintf("%s!$D$2:$D$%d", summarySheetName, lastRow),
			},
			{
				Name:       fmt.Sprintf("%s!$E$1", summarySheetName), // Limit Memory
				Categories: fmt.Sprintf("%s!$A$2:$A$%d", summarySheetName, lastRow),
				Values:     fmt.Sprintf("%s!$E$2:$E$%d", summarySheetName, lastRow),
			},
		},
		Title: []excelize.RichTextRun{
			{Text: "Memory Resources by Namespace (Mi)"},
		},
		Legend: excelize.ChartLegend{
			Position: "top",
		},
		Dimension: excelize.ChartDimension{
			Width:  width,
			Height: height / 2, // Half height for each chart
		},
	}); err != nil {
		return fmt.Errorf("failed to add Memory chart: %w", err)
	}

	logrus.Infof("Created chart sheet with %d namespaces (size: %dx%d)", len(namespaceTotals), width, height)
	return nil
}

// Enhanced error context for row operations
func setRowWithContext(f *excelize.File, sheetName string, row int, data []interface{}, context string) error {
	cellName, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return fmt.Errorf("failed to get cell name for row %d in %s: %w", row, context, err)
	}

	if err := f.SetSheetRow(sheetName, cellName, &data); err != nil {
		return fmt.Errorf("failed to set row data for %s at row %d: %w", context, row, err)
	}

	return nil
}

// Memory usage monitoring
func logMemoryUsage(stage string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	logrus.Debugf("Memory usage at %s: Alloc=%d KB, Sys=%d KB",
		stage, m.Alloc/1024, m.Sys/1024)
}

// Data validation and warnings
func validateAndWarnResources(namespaceTotals map[string]struct {
	reqCPU, limCPU int64
	reqMem, limMem int64
}, nodeTotals map[string]struct {
	podCount       int
	reqCPU, limCPU int64
	reqMem, limMem int64
}, containerCount int) {

	var warnings []string

	// Check for namespaces without limits
	noLimitsNS := 0
	for ns, totals := range namespaceTotals {
		if totals.limCPU == 0 && totals.limMem == 0 {
			noLimitsNS++
			if noLimitsNS <= 3 { // Show first 3
				warnings = append(warnings, fmt.Sprintf("Namespace '%s' has no resource limits", ns))
			}
		}
	}
	if noLimitsNS > 3 {
		warnings = append(warnings, fmt.Sprintf("... and %d more namespaces without limits", noLimitsNS-3))
	}

	// Check for unbalanced nodes
	if len(nodeTotals) > 1 {
		var podCounts []int
		for _, totals := range nodeTotals {
			podCounts = append(podCounts, totals.podCount)
		}

		// Simple imbalance check
		minPods, maxPods := podCounts[0], podCounts[0]
		for _, count := range podCounts {
			if count < minPods {
				minPods = count
			}
			if count > maxPods {
				maxPods = count
			}
		}

		if maxPods > minPods*2 {
			warnings = append(warnings, fmt.Sprintf("Pod distribution imbalanced: %d-%d pods per node", minPods, maxPods))
		}
	}

	// Log warnings
	if len(warnings) > 0 {
		logrus.Warn("Resource validation warnings:")
		for _, warning := range warnings {
			logrus.Warn("  - " + warning)
		}
	}

	logrus.Infof("Validation complete: %d namespaces, %d nodes, %d containers",
		len(namespaceTotals), len(nodeTotals), containerCount)
}

// Bold style for totals
func getBoldStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	return style
}

// Bold number style for totals
func getBoldNumberStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		NumFmt: 2, // 0.0 format
	})
	return style
}

// Percentage calculation helper
func calculatePercentage(part, total int64) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(total)*100)
}

// Data Science Insights Sheet
func createInsightsSheet(f *excelize.File, namespaceTotals map[string]struct {
	reqCPU, limCPU int64
	reqMem, limMem int64
}, nodeTotals map[string]struct {
	podCount       int
	reqCPU, limCPU int64
	reqMem, limMem int64
}, containerCount int, sheetName string) error {

	_, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create insights sheet: %w", err)
	}

	row := 1

	// Title
	f.SetCellValue(sheetName, "A1", "📊 KUBERNETES RESOURCE INSIGHTS")
	f.SetCellStyle(sheetName, "A1", "A1", getTitleStyle(f))
	row += 3

	// 1. Resource Efficiency Analysis
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "🎯 RESOURCE EFFICIENCY ANALYSIS")
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), getHeaderStyle(f))
	row += 2

	var totalReqCPU, totalLimCPU, totalReqMem, totalLimMem int64
	var overProvisionedNS, underProvisionedNS, balancedNS int

	for _, totals := range namespaceTotals {
		totalReqCPU += totals.reqCPU
		totalLimCPU += totals.limCPU
		totalReqMem += totals.reqMem
		totalLimMem += totals.limMem

		// Efficiency classification
		cpuEff := float64(totals.reqCPU) / float64(totals.limCPU) * 100
		memEff := float64(totals.reqMem) / float64(totals.limMem) * 100
		avgEff := (cpuEff + memEff) / 2

		if avgEff < 50 {
			overProvisionedNS++
		} else if avgEff > 80 {
			underProvisionedNS++
		} else {
			balancedNS++
		}
	}

	clusterCPUEff := float64(totalReqCPU) / float64(totalLimCPU) * 100
	clusterMemEff := float64(totalReqMem) / float64(totalLimMem) * 100

	insights := [][]interface{}{
		{"Cluster CPU Efficiency", fmt.Sprintf("%.1f%%", clusterCPUEff), getEfficiencyRating(clusterCPUEff)},
		{"Cluster Memory Efficiency", fmt.Sprintf("%.1f%%", clusterMemEff), getEfficiencyRating(clusterMemEff)},
		{"Over-provisioned Namespaces", overProvisionedNS, "< 50% efficiency"},
		{"Well-balanced Namespaces", balancedNS, "50-80% efficiency"},
		{"Under-provisioned Namespaces", underProvisionedNS, "> 80% efficiency"},
		{"Potential CPU Savings", fmt.Sprintf("%.1f cores", float64(totalLimCPU-totalReqCPU)/1000), "If limits = requests"},
		{"Potential Memory Savings", fmt.Sprintf("%.1f Gi", float64(totalLimMem-totalReqMem)/(1024*1024*1024)), "If limits = requests"},
	}

	for _, insight := range insights {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), insight[0])
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), insight[1])
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), insight[2])
		row++
	}
	row += 2

	// 2. Node Distribution Analysis
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "🏗️ NODE DISTRIBUTION ANALYSIS")
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), getHeaderStyle(f))
	row += 2

	var podCounts []int
	var nodeCPUs, nodeMemories []int64
	for _, totals := range nodeTotals {
		podCounts = append(podCounts, totals.podCount)
		nodeCPUs = append(nodeCPUs, totals.reqCPU)
		nodeMemories = append(nodeMemories, totals.reqMem)
	}

	nodeInsights := [][]interface{}{
		{"Total Nodes", len(nodeTotals), ""},
		{"Average Pods per Node", fmt.Sprintf("%.1f", average(podCounts)), ""},
		{"Pod Distribution StdDev", fmt.Sprintf("%.1f", stdDev(podCounts)), "Lower = better balance"},
		{"Most Loaded Node", fmt.Sprintf("%d pods", max(podCounts)), ""},
		{"Least Loaded Node", fmt.Sprintf("%d pods", min(podCounts)), ""},
		{"Load Balance Score", getBalanceScore(podCounts), "0-100 (100 = perfect)"},
	}

	for _, insight := range nodeInsights {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), insight[0])
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), insight[1])
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), insight[2])
		row++
	}
	row += 2

	// 3. Recommendations
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "💡 OPTIMIZATION RECOMMENDATIONS")
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), getHeaderStyle(f))
	row += 2

	recommendations := generateRecommendations(clusterCPUEff, clusterMemEff, overProvisionedNS, underProvisionedNS, getBalanceScoreValue(podCounts))

	for _, rec := range recommendations {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "•")
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), rec)
		row++
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 25)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 30)

	return nil
}

// Helper functions for data science calculations
func average(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func stdDev(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	avg := average(values)
	var sum float64
	for _, v := range values {
		sum += (float64(v) - avg) * (float64(v) - avg)
	}
	return math.Sqrt(sum / float64(len(values)))
}

func max(values []int) int {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func min(values []int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

func getBalanceScore(values []int) string {
	return fmt.Sprintf("%.0f", getBalanceScoreValue(values))
}

func getBalanceScoreValue(values []int) float64 {
	if len(values) <= 1 {
		return 100
	}
	std := stdDev(values)
	avg := average(values)
	if avg == 0 {
		return 100
	}
	cv := std / avg                  // Coefficient of variation
	return math.Max(0, 100-(cv*100)) // Lower CV = better balance
}

func getEfficiencyRating(eff float64) string {
	if eff >= 80 {
		return "⚠️ Under-provisioned"
	}
	if eff >= 60 {
		return "✅ Well-balanced"
	}
	if eff >= 40 {
		return "⚡ Over-provisioned"
	}
	return "🔴 Severely over-provisioned"
}

func generateRecommendations(cpuEff, memEff float64, overProv, underProv int, balanceScore float64) []string {
	var recs []string

	if cpuEff < 50 {
		recs = append(recs, "Consider reducing CPU limits - cluster is over-provisioned")
	}
	if memEff < 50 {
		recs = append(recs, "Consider reducing Memory limits - cluster is over-provisioned")
	}
	if cpuEff > 80 {
		recs = append(recs, "⚠️ CPU limits too tight - risk of throttling")
	}
	if memEff > 80 {
		recs = append(recs, "⚠️ Memory limits too tight - risk of OOM kills")
	}
	if overProv > underProv {
		recs = append(recs, "Focus on right-sizing over-provisioned namespaces first")
	}
	if balanceScore < 70 {
		recs = append(recs, "Consider pod anti-affinity rules for better node distribution")
	}
	if len(recs) == 0 {
		recs = append(recs, "✅ Cluster resource allocation looks well-balanced!")
	}

	return recs
}

func getTitleStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16},
	})
	return style
}

func getHeaderStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 12},
	})
	return style
}
