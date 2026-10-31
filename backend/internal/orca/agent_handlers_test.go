package orca

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListAgentRunsReportsProviderReadinessWithoutExposingKey(t *testing.T) {
	const secret = "test-only-groq-key"
	for _, test := range []struct {
		name      string
		key       string
		available bool
	}{
		{name: "configured", key: secret, available: true},
		{name: "blank", key: "  ", available: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			api := NewAPI(&Store{state: NewState()}, ServerConfig{GroqAPIKey: test.key})
			request := httptest.NewRequest("GET", "/api/v1/agent/runs", nil)
			response := httptest.NewRecorder()
			api.listAgentRuns(response, request, Authenticated{Actor: Actor{BusinessID: "business-a", UserID: "user-a", Role: "admin"}})
			if response.Code != 200 {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["available"] != test.available {
				t.Fatalf("available = %#v, want %v", body["available"], test.available)
			}
			if _, ok := body["items"].([]any); !ok {
				t.Fatalf("items are missing from response: %#v", body)
			}
			if strings.Contains(response.Body.String(), secret) {
				t.Fatal("provider credential was exposed")
			}
		})
	}
}
