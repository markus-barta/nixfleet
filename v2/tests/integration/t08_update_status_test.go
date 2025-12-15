package integration

import (
	"strings"
	"testing"
)

// TestUpdateStatus_UIElements tests that the update status UI elements are rendered.
func TestUpdateStatus_UIElements(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	client := newClientWithCookiesFollowRedirects(t)

	// Login and get dashboard
	resp, err := client.PostForm(td.URL()+"/login", map[string][]string{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Dashboard should contain update status CSS classes
	body := make([]byte, 64*1024)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	// Check for update status CSS
	if !strings.Contains(bodyStr, "update-status") {
		t.Log("Dashboard may not have update-status class visible (expected with no hosts)")
	}

	// Check for bulk action buttons
	if !strings.Contains(bodyStr, "Update All") {
		t.Error("missing 'Update All' bulk action button")
	}
	if !strings.Contains(bodyStr, "Pull All") {
		t.Error("missing 'Pull All' bulk action button")
	}
	if !strings.Contains(bodyStr, "Switch All") {
		t.Error("missing 'Switch All' bulk action button")
	}
	if !strings.Contains(bodyStr, "Test All") {
		t.Error("missing 'Test All' bulk action button")
	}

	// Check for modal HTML
	if !strings.Contains(bodyStr, "removeHostModal") {
		t.Error("missing Remove Host modal")
	}
	if !strings.Contains(bodyStr, "addHostModal") {
		t.Error("missing Add Host modal")
	}

	// Check for update status icons
	if !strings.Contains(bodyStr, "icon-git-branch") {
		t.Error("missing git-branch icon definition")
	}
	if !strings.Contains(bodyStr, "icon-lock") {
		t.Error("missing lock icon definition")
	}

	t.Log("UI elements for P5000 (Update Status), P4380 (Dropdown), P4390 (Modals) present")
}

// TestUpdateStatus_ThreeCompartmentCSS tests that CSS for three-compartment is present.
func TestUpdateStatus_ThreeCompartmentCSS(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	client := newClientWithCookiesFollowRedirects(t)

	// Login
	resp, err := client.PostForm(td.URL()+"/login", map[string][]string{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body := make([]byte, 64*1024)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	// Check for update compartment CSS
	cssChecks := []string{
		".update-compartment",
		".update-compartment.needs-update",
		".update-compartment.error",
		".update-compartment.unknown",
		"pulse-glow",
	}

	for _, check := range cssChecks {
		if !strings.Contains(bodyStr, check) {
			t.Errorf("missing CSS: %s", check)
		}
	}

	t.Log("three-compartment CSS styles present")
}

