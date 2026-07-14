package apperr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/drswith/easy-proxy-switch-cli/internal/apperr"
)

func TestMisconfig(t *testing.T) {
	err := apperr.Misconfigf("bad %s", "mode")
	if !apperr.IsMisconfig(err) {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "bad mode") {
		t.Fatal(err)
	}
	if apperr.IsMisconfig(errors.New("x")) {
		t.Fatal("generic should not be misconfig")
	}
}
