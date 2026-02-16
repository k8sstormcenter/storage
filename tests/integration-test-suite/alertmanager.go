package integration_test_suite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

const defaultAlertManagerURL = "http://localhost:9093"

// Alert structure based on the expected JSON format from Alertmanager.
type Alert struct {
	Labels map[string]string `json:"labels"`
}

func getAlertManagerURL() string {
	if url := os.Getenv("ALERTMANAGER_URL"); url != "" {
		return url
	}
	return defaultAlertManagerURL
}

// getAlerts retrieves active alerts from Alertmanager filtered by namespace.
func getAlerts(namespace string) ([]Alert, error) {
	url := getAlertManagerURL()
	endpoint := fmt.Sprintf("%s/api/v2/alerts?active=true", url)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to alertmanager at %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alertmanager http error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}

	var alerts []Alert
	if err := json.Unmarshal(body, &alerts); err != nil {
		return nil, fmt.Errorf("json parsing error: %v", err)
	}

	// Filter by alertname and namespace
	var filtered []Alert
	for _, a := range alerts {
		if a.Labels["alertname"] == "KubescapeRuleViolated" && a.Labels["namespace"] == namespace {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}

// assertAlertPresent checks that an alert matching the given rule name and command exists.
func assertAlertPresent(t *testing.T, alerts []Alert, ruleName, command, containerName string) {
	t.Helper()
	for _, a := range alerts {
		if a.Labels["rule_name"] == ruleName &&
			a.Labels["comm"] == command &&
			a.Labels["container_name"] == containerName {
			return
		}
	}
	t.Errorf("expected alert with rule_name=%q, comm=%q, container_name=%q not found", ruleName, command, containerName)
	for _, a := range alerts {
		t.Logf("  alert labels: %v", a.Labels)
	}
}

// assertAlertAbsent checks that no alert matching the given rule name and command exists.
func assertAlertAbsent(t *testing.T, alerts []Alert, ruleName, command, containerName string) {
	t.Helper()
	for _, a := range alerts {
		if a.Labels["rule_name"] == ruleName &&
			a.Labels["comm"] == command &&
			a.Labels["container_name"] == containerName {
			t.Errorf("unexpected alert with rule_name=%q, comm=%q, container_name=%q found", ruleName, command, containerName)
			return
		}
	}
}

// startAlertManagerPortForward starts kubectl port-forward for AlertManager.
// Returns the exec.Cmd so the caller can kill it when done.
func startAlertManagerPortForward(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("kubectl", "port-forward", "svc/alertmanager-operated", "9093:9093", "-n", "monitoring")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start alertmanager port-forward: %v", err)
	}
	// Wait for port-forward to establish
	time.Sleep(5 * time.Second)

	// Verify connectivity
	url := getAlertManagerURL()
	resp, err := http.Get(fmt.Sprintf("%s/api/v2/status", url))
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("AlertManager not reachable after port-forward: %v", err)
	}
	resp.Body.Close()

	t.Logf("AlertManager port-forward established at %s", url)
	return cmd
}
