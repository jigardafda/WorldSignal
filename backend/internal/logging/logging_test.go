package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelsRouteAndFormat(t *testing.T) {
	var out, errw bytes.Buffer
	l := &Logger{scope: "test", out: &out, errw: &errw}

	l.Info("hello")
	l.Debug("dbg", 42)
	l.Warn("careful")
	l.Error("boom", "detail")

	o := out.String()
	e := errw.String()

	if !strings.Contains(o, "INFO  [test] hello") {
		t.Fatalf("info line missing: %q", o)
	}
	if !strings.Contains(o, "DEBUG [test] dbg 42") {
		t.Fatalf("debug+extra missing: %q", o)
	}
	if !strings.Contains(e, "WARN  [test] careful") {
		t.Fatalf("warn should go to err: %q", e)
	}
	if !strings.Contains(e, "ERROR [test] boom detail") {
		t.Fatalf("error+extra missing: %q", e)
	}
}

func TestNewUsesStdStreams(t *testing.T) {
	l := New("scope")
	if l.out == nil || l.errw == nil || l.scope != "scope" {
		t.Fatal("New should wire std streams and scope")
	}
}
