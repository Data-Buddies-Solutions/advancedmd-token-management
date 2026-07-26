package safelog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriterProducesStructuredJSON(t *testing.T) {
	t.Run("wraps a safe event message", func(t *testing.T) {
		var output bytes.Buffer
		input := []byte("patient-mutation operation=create category=failed\n")

		n, err := NewWriter(&output).Write(input)

		if err != nil || n != len(input) {
			t.Fatalf("Write() = %d, %v", n, err)
		}
		var entry map[string]any
		if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
			t.Fatalf("output is not JSON: %q: %v", output.String(), err)
		}
		if entry["message"] != strings.TrimSpace(string(input)) {
			t.Fatalf("message = %v, want original safe event", entry["message"])
		}
	})

	t.Run("preserves a structured request entry", func(t *testing.T) {
		var output bytes.Buffer
		input := []byte(`{"route_template":"/live","outcome_category":"success"}` + "\n")

		_, err := NewWriter(&output).Write(input)

		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if output.String() != string(input) {
			t.Fatalf("output = %q, want %q", output.String(), input)
		}
	})

	t.Run("wraps valid JSON that is not an object", func(t *testing.T) {
		var output bytes.Buffer

		_, err := NewWriter(&output).Write([]byte(`"safe event"` + "\n"))

		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		var entry map[string]any
		if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
			t.Fatalf("output is not a JSON object: %q: %v", output.String(), err)
		}
	})
}
