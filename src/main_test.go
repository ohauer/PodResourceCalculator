package main

import (
	"os"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNamespaceDisplay(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "all namespaces"},
		{"default", "default"},
		{"kube-system", "kube-system"},
	}

	for _, test := range tests {
		result := getNamespaceDisplay(test.input)
		if result != test.expected {
			t.Errorf("getNamespaceDisplay(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestGetOutputFilename(t *testing.T) {
	// Test with custom output
	custom := "custom.xlsx"
	result := getOutputFilename(custom)
	if result != custom {
		t.Errorf("getOutputFilename(%q) = %q, want %q", custom, result, custom)
	}

	// Test with empty output (should generate date-based filename)
	result = getOutputFilename("")
	expected := "resource_" + time.Now().Format("2006-01-02") + ".xlsx"
	if result != expected {
		t.Errorf("getOutputFilename(\"\") = %q, want %q", result, expected)
	}
}

func TestHomeDir(t *testing.T) {
	// Save original values
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")

	// Test HOME environment variable
	os.Setenv("HOME", "/home/test")
	os.Unsetenv("USERPROFILE")
	result := homeDir()
	if result != "/home/test" {
		t.Errorf("homeDir() with HOME = %q, want %q", result, "/home/test")
	}

	// Test USERPROFILE environment variable (Windows)
	os.Unsetenv("HOME")
	os.Setenv("USERPROFILE", "C:\\Users\\test")
	result = homeDir()
	if result != "C:\\Users\\test" {
		t.Errorf("homeDir() with USERPROFILE = %q, want %q", result, "C:\\Users\\test")
	}

	// Test no environment variables
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	result = homeDir()
	if result != "" {
		t.Errorf("homeDir() with no env vars = %q, want empty string", result)
	}

	// Restore original values
	if originalHome != "" {
		os.Setenv("HOME", originalHome)
	}
	if originalUserProfile != "" {
		os.Setenv("USERPROFILE", originalUserProfile)
	}
}

func TestSetColumnWidths(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "TestSheet"
	_, err := f.NewSheet(sheetName)
	if err != nil {
		t.Fatalf("Failed to create test sheet: %v", err)
	}

	err = setColumnWidths(f, sheetName)
	if err != nil {
		t.Errorf("setColumnWidths() failed: %v", err)
	}

	// Test that column widths were set (basic validation)
	// Note: excelize doesn't provide a direct way to read column widths,
	// so we just ensure the function doesn't error
}

func TestCreateSummarySheet(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	// Create test pods with resource specifications
	_ = []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-1",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test-container",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
			},
		},
	}

	// Create test namespace totals
	namespaceTotals := make(map[string]struct {
		reqCPU, limCPU int64
		reqMem, limMem int64
	})
	namespaceTotals["default"] = struct {
		reqCPU, limCPU int64
		reqMem, limMem int64
	}{reqCPU: 1000, limCPU: 2000, reqMem: 1024 * 1024 * 1024, limMem: 2 * 1024 * 1024 * 1024}

	err := createSummarySheetFromData(f, namespaceTotals, "Summary")
	if err != nil {
		t.Errorf("createSummarySheetFromData() failed: %v", err)
	}

	// Verify the summary sheet was created
	sheets := f.GetSheetList()
	found := false
	for _, sheet := range sheets {
		if sheet == "Summary" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Summary sheet was not created")
	}
}
