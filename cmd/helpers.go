package cmd

import (
	"os/exec"
	"syscall"

	"github.com/drswith/easy-proxy-cli/internal/config"
	"github.com/drswith/easy-proxy-cli/internal/doctor"
	"github.com/drswith/easy-proxy-cli/internal/proxy"
	"github.com/drswith/easy-proxy-cli/internal/shell"
	tomllib "github.com/pelletier/go-toml/v2"
)

func shellDetect(name string) (shell.Kind, error) {
	return shell.Detect(name)
}

func hookScript(kind shell.Kind, bin string) (string, error) {
	return shell.HookScript(kind, bin)
}

func resolveProxy(cfg config.Config, opt proxy.ResolveOptions) (proxy.Resolved, error) {
	return proxy.Resolve(cfg, opt)
}

func runDoctor(httpProxy, socksProxy, probe string) doctor.Report {
	return doctor.Run(httpProxy, socksProxy, probe, 0)
}

func marshalConfig(cfg config.Config) (string, error) {
	b, err := tomllib.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		return -1
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
		if ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ws.ExitStatus()
	}
	code := ee.ExitCode()
	if code < 0 {
		return -1
	}
	return code
}
