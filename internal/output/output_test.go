package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drswith/easy-proxy-switch-cli/internal/output"
)

func TestJSONAndHumanAndScript(t *testing.T) {
	var out, errBuf bytes.Buffer
	w := output.Writer{Out: &out, Err: &errBuf, Mode: output.Mode{JSON: true}}
	if err := w.JSON(map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"ok"`) {
		t.Fatalf("json=%s", out.String())
	}

	out.Reset()
	errBuf.Reset()
	w = output.Writer{Out: &out, Err: &errBuf, Mode: output.Mode{}}
	w.Human("hello %s", "world")
	w.Script("export A=1\n")
	if !strings.Contains(errBuf.String(), "hello world") {
		t.Fatalf("stderr=%s", errBuf.String())
	}
	if out.String() != "export A=1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestExitCodesStable(t *testing.T) {
	if output.ExitOK != 0 || output.ExitError != 1 || output.ExitMisconfig != 2 || output.ExitUnavailable != 3 {
		t.Fatal("exit codes changed")
	}
}
