package dynamicpathdetectortests

import (
	"testing"

	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
)

func TestCompareExecArgs(t *testing.T) {
	tests := []struct {
		name        string
		profileArgs []string
		runtimeArgs []string
		expected    bool
	}{
		{
			name:        "Exact match",
			profileArgs: []string{"-la"},
			runtimeArgs: []string{"-la"},
			expected:    true,
		},
		{
			name:        "Exact mismatch",
			profileArgs: []string{"-la", "/tmp"},
			runtimeArgs: []string{"-la", "/home"},
			expected:    false,
		},
		{
			name:        "Wildcard matches any args",
			profileArgs: []string{"*"},
			runtimeArgs: []string{"-la", "/tmp", "--color"},
			expected:    true,
		},
		{
			name:        "Wildcard matches empty args",
			profileArgs: []string{"*"},
			runtimeArgs: []string{},
			expected:    true,
		},
		{
			name:        "Prefix plus wildcard",
			profileArgs: []string{"-X", "POST", "*"},
			runtimeArgs: []string{"-X", "POST", "https://api.example.com", "--header", "Content-Type: application/json"},
			expected:    true,
		},
		{
			name:        "Prefix plus wildcard zero trailing",
			profileArgs: []string{"-p", "*"},
			runtimeArgs: []string{"-p"},
			expected:    true,
		},
		{
			name:        "Prefix plus wildcard wrong prefix",
			profileArgs: []string{"-X", "GET", "*"},
			runtimeArgs: []string{"-X", "POST", "https://api.example.com"},
			expected:    false,
		},
		{
			name:        "Dynamic matches one arg",
			profileArgs: []string{"-s", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"-s", "http://any.example.com"},
			expected:    true,
		},
		{
			name:        "Dynamic does not match zero",
			profileArgs: []string{"-s", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"-s"},
			expected:    false,
		},
		{
			name:        "Multiple dynamic",
			profileArgs: []string{"-p", dynamicpathdetector.DynamicIdentifier, "--out", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"-p", "8080", "--out", "/log"},
			expected:    true,
		},
		{
			name:        "Dynamic then wildcard",
			profileArgs: []string{dynamicpathdetector.DynamicIdentifier, "*"},
			runtimeArgs: []string{"first", "second", "third"},
			expected:    true,
		},
		{
			name:        "Dynamic then wildcard fails empty",
			profileArgs: []string{dynamicpathdetector.DynamicIdentifier, "*"},
			runtimeArgs: []string{},
			expected:    false,
		},
		{
			name:        "Mixed literal dynamic literal wildcard",
			profileArgs: []string{"--mode", dynamicpathdetector.DynamicIdentifier, "--config", "*"},
			runtimeArgs: []string{"--mode", "prod", "--config", "f.yaml", "-v"},
			expected:    true,
		},
		{
			name:        "Mixed wrong literal",
			profileArgs: []string{"--mode", dynamicpathdetector.DynamicIdentifier, "--config", "*"},
			runtimeArgs: []string{"--mode", "prod", "--wrong", "f.yaml"},
			expected:    false,
		},
		{
			name:        "Both empty",
			profileArgs: []string{},
			runtimeArgs: []string{},
			expected:    true,
		},
		{
			name:        "Both nil",
			profileArgs: nil,
			runtimeArgs: nil,
			expected:    true,
		},
		{
			name:        "Empty profile non-empty runtime",
			profileArgs: []string{},
			runtimeArgs: []string{"-la"},
			expected:    false,
		},
		{
			name:        "Real-world kubectl wildcard",
			profileArgs: []string{"kubectl", "get", "pods", "-n", dynamicpathdetector.DynamicIdentifier, "*"},
			runtimeArgs: []string{"kubectl", "get", "pods", "-n", "kube-system", "--output=json"},
			expected:    true,
		},
		{
			name:        "Real-world iptables complex",
			profileArgs: []string{"-t", dynamicpathdetector.DynamicIdentifier, "-A", dynamicpathdetector.DynamicIdentifier, "-j", "*"},
			runtimeArgs: []string{"-t", "nat", "-A", "PREROUTING", "-j", "DNAT", "--to-dest", "10.0.0.1"},
			expected:    true,
		},
		{
			name:        "Real-world curl user allowlist",
			profileArgs: []string{"/usr/bin/curl", "-s", "*"},
			runtimeArgs: []string{"/usr/bin/curl", "-s", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"},
			expected:    true,
		},
		// --- Exec-specific scenarios (strict, dynamic, wildcard boundary cases) ---
		{
			name:        "Strict mkdir -p with exact dir",
			profileArgs: []string{"/bin/mkdir", "-p", "/var/lock/apache2"},
			runtimeArgs: []string{"/bin/mkdir", "-p", "/var/lock/apache2"},
			expected:    true,
		},
		{
			name:        "Strict mkdir -p with different dir",
			profileArgs: []string{"/bin/mkdir", "-p", "/var/lock/apache2"},
			runtimeArgs: []string{"/bin/mkdir", "-p", "/var/log/apache2"},
			expected:    false,
		},
		{
			name:        "Strict rm -f exact path",
			profileArgs: []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"},
			runtimeArgs: []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"},
			expected:    true,
		},
		{
			name:        "Strict rm different flags and path",
			profileArgs: []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"},
			runtimeArgs: []string{"/bin/rm", "-rf", "/tmp"},
			expected:    false,
		},
		{
			name:        "Dynamic mkdir -p matches any dir",
			profileArgs: []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"/bin/mkdir", "-p", "/var/log"},
			expected:    true,
		},
		{
			name:        "Dynamic mkdir missing -p flag",
			profileArgs: []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"/bin/mkdir", "/var/log"},
			expected:    false,
		},
		{
			name:        "Dynamic mkdir -p extra arg beyond dynamic",
			profileArgs: []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"/bin/mkdir", "-p", "-v", "/var/log"},
			expected:    false,
		},
		{
			name:        "Dynamic requires at least one arg",
			profileArgs: []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"/bin/mkdir", "-p"},
			expected:    false,
		},
		{
			name:        "Dynamic dirname matches any path",
			profileArgs: []string{"/usr/bin/dirname", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"/usr/bin/dirname", "/var/log"},
			expected:    true,
		},
		{
			name:        "Dynamic dirname requires one arg",
			profileArgs: []string{"/usr/bin/dirname", dynamicpathdetector.DynamicIdentifier},
			runtimeArgs: []string{"/usr/bin/dirname"},
			expected:    false,
		},
		{
			name:        "Wildcard echo hello matches zero trailing",
			profileArgs: []string{"/bin/echo", "hello", "*"},
			runtimeArgs: []string{"/bin/echo", "hello"},
			expected:    true,
		},
		{
			name:        "Wildcard echo hello matches one trailing",
			profileArgs: []string{"/bin/echo", "hello", "*"},
			runtimeArgs: []string{"/bin/echo", "hello", "world"},
			expected:    true,
		},
		{
			name:        "Wildcard echo hello rejects wrong literal",
			profileArgs: []string{"/bin/echo", "hello", "*"},
			runtimeArgs: []string{"/bin/echo", "goodbye"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dynamicpathdetector.CompareExecArgs(tt.profileArgs, tt.runtimeArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}
