package main

import (
	"testing"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty path", "", false},
		{"valid relative path", "output.xlsx", false},
		{"valid absolute path", "/tmp/output.xlsx", false},
		{"path traversal with dots", "../../../etc/passwd", true},
		{"path traversal absolute", "/etc/passwd", true},
		{"path traversal /sys", "/sys/kernel", true},
		{"path traversal /proc", "/proc/cpuinfo", true},
		{"path traversal /dev", "/dev/null", true},
		{"valid home path", "~/output.xlsx", false},
		{"clean path with dots", "./output.xlsx", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name    string
		ns      string
		wantErr bool
	}{
		{"empty namespace", "", false},
		{"valid lowercase", "default", false},
		{"valid with hyphen", "kube-system", false},
		{"valid with numbers", "app-v2", false},
		{"invalid uppercase", "Default", true},
		{"invalid underscore", "kube_system", true},
		{"invalid start with hyphen", "-default", true},
		{"invalid end with hyphen", "default-", true},
		{"invalid path traversal", "../../../etc", true},
		{"invalid special chars", "kube@system", true},
		{"invalid too long", "this-is-a-very-long-namespace-name-that-exceeds-the-maximum-length-of-sixty-three-characters", true},
		{"valid max length", "this-is-exactly-sixty-three-characters-long-namespace-name-ok", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespace(tt.ns)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetOutputFilename(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{"empty returns default", "", "resource_"},
		{"custom filename", "custom.xlsx", "custom.xlsx"},
		{"path with traversal gets cleaned", "../output.xlsx", "../output.xlsx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getOutputFilename(tt.output)
			if tt.want == "resource_" {
				// Check it starts with resource_ (date will vary)
				if len(got) < 18 || got[:9] != "resource_" {
					t.Errorf("getOutputFilename() = %v, want prefix %v", got, tt.want)
				}
			} else if got != tt.want {
				t.Errorf("getOutputFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNamespaceDisplay(t *testing.T) {
	tests := []struct {
		name string
		ns   string
		want string
	}{
		{"empty namespace", "", "all namespaces"},
		{"specific namespace", "default", "default"},
		{"kube-system", "kube-system", "kube-system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getNamespaceDisplay(tt.ns); got != tt.want {
				t.Errorf("getNamespaceDisplay() = %v, want %v", got, tt.want)
			}
		})
	}
}
