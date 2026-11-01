package logx_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/logx"
)

func TestLevels(t *testing.T) {
	var quiet, loud bytes.Buffer
	logx.New(false, false, &quiet).Debug("hidden")
	logx.New(true, false, &loud).Debug("shown")
	if quiet.Len() != 0 {
		t.Errorf("debug leaked at info level: %s", quiet.String())
	}
	if !strings.Contains(loud.String(), "shown") {
		t.Errorf("debug missing when verbose: %s", loud.String())
	}
}

func TestJSONHandler(t *testing.T) {
	var buf bytes.Buffer
	logx.New(false, true, &buf).Info("route", "pm", "alpha")
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %v: %s", err, buf.String())
	}
	if rec["pm"] != "alpha" || rec["msg"] != "route" {
		t.Errorf("record = %v", rec)
	}
}
