package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/drswith/easy-proxy-cli/internal/config"
	"github.com/drswith/easy-proxy-cli/internal/shell"
)

const (
	MarkerBegin = "# >>> easy-proxy-cli >>>"
	MarkerEnd   = "# <<< easy-proxy-cli <<<"
)

// Target is one shell rc/profile file to modify.
type Target struct {
	Shell   shell.Kind `json:"shell"`
	Path    string     `json:"path"`
	Source  string     `json:"source"` // shell_env | existing_rc | explicit | powershell_path
	Existed bool       `json:"existed"`
}

// Options controls setup behavior.
type Options struct {
	// Shells limits targets to these kinds (e.g. "zsh", "bash"). Empty = auto.
	Shells []string
	// BinPath is the ezp binary path embedded in hooks. Empty = look up executable.
	BinPath string
	// AllDetected writes to $SHELL target plus every existing supported rc.
	// Default true.
	AllDetected bool
	// CreateMissing creates rc/profile files that do not exist yet.
	CreateMissing bool
	// DryRun only reports actions.
	DryRun bool
	// Uninstall removes managed blocks instead of writing them.
	Uninstall bool
	// InitConfig writes default ~/.easy-proxy/config.toml if missing.
	InitConfig bool
}

// Result summarizes setup work.
type Result struct {
	Binary  string         `json:"binary"`
	Config  string         `json:"config,omitempty"`
	Targets []TargetResult `json:"targets"`
	Hints   []string       `json:"hints,omitempty"`
}

// TargetResult is per-file outcome.
type TargetResult struct {
	Target Target `json:"target"`
	Action string `json:"action"` // written|updated|removed|skipped|would_write|would_remove
	Error  string `json:"error,omitempty"`
}

// Run discovers targets and applies hook blocks.
func Run(opt Options) (Result, error) {
	bin := opt.BinPath
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			bin = "ezp"
		} else {
			bin, _ = filepath.Abs(exe)
		}
	}

	targets, err := DiscoverTargets(opt)
	if err != nil {
		return Result{}, err
	}

	res := Result{Binary: bin, Targets: make([]TargetResult, 0, len(targets))}

	if opt.InitConfig && !opt.Uninstall {
		if opt.DryRun {
			path, err := config.Path()
			if err != nil {
				return Result{}, err
			}
			res.Config = path
			if !fileExists(path) {
				res.Hints = append(res.Hints, "would create config "+path)
			}
		} else {
			path, created, err := ensureConfig()
			if err != nil {
				return Result{}, err
			}
			res.Config = path
			if created {
				res.Hints = append(res.Hints, "created config "+path)
			}
		}
	}

	for _, t := range targets {
		tr := TargetResult{Target: t}
		if opt.Uninstall {
			action, err := removeHook(t.Path, opt.DryRun)
			tr.Action = action
			if err != nil {
				tr.Error = err.Error()
			}
		} else {
			script, err := shell.HookScript(t.Shell, bin)
			if err != nil {
				tr.Action = "skipped"
				tr.Error = err.Error()
				res.Targets = append(res.Targets, tr)
				continue
			}
			block := wrapBlock(t.Shell, script)
			// Only CreateMissing (or an already-existing file) may create/write.
			// Target Source must not bypass --create-missing=false.
			create := opt.CreateMissing || t.Existed
			action, err := writeHook(t.Path, block, opt.DryRun, create)
			tr.Action = action
			if err != nil {
				tr.Error = err.Error()
			}
		}
		res.Targets = append(res.Targets, tr)
	}

	res.Hints = append(res.Hints, reloadHints(targets)...)
	if err := firstTargetError(res.Targets); err != nil {
		return res, err
	}
	return res, nil
}

func firstTargetError(targets []TargetResult) error {
	for _, t := range targets {
		if t.Error != "" {
			return fmt.Errorf("%s: %s", t.Target.Path, t.Error)
		}
	}
	return nil
}

func wrapBlock(kind shell.Kind, script string) string {
	script = strings.TrimRight(script, "\n") + "\n"
	switch kind {
	case shell.Fish:
		// fish uses # comments too
		return MarkerBegin + "\n" + script + MarkerEnd + "\n"
	case shell.PowerShell:
		return MarkerBegin + "\n" + script + MarkerEnd + "\n"
	default:
		return MarkerBegin + "\n" + script + MarkerEnd + "\n"
	}
}

func writeHook(path, block string, dryRun, create bool) (string, error) {
	data, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return "skipped", err
	}
	if !existed && !create {
		// Soft skip: --create-missing=false deliberately leaves missing rcs alone.
		return "skipped", nil
	}

	content := ""
	if existed {
		content = string(data)
	}
	next, changed, err := upsertBlock(content, block)
	if err != nil {
		return "skipped", err
	}
	if !changed {
		return "skipped", nil
	}
	action := "updated"
	if !existed {
		action = "written"
	}
	if dryRun {
		if !existed {
			return "would_write", nil
		}
		return "would_update", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "skipped", err
	}
	// preserve mode when possible
	mode := os.FileMode(0o644)
	if existed {
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
	}
	if err := os.WriteFile(path, []byte(next), mode); err != nil {
		return "skipped", err
	}
	return action, nil
}

func removeHook(path string, dryRun bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "skipped", nil
		}
		return "skipped", err
	}
	next, changed, err := stripBlock(string(data))
	if err != nil {
		return "skipped", err
	}
	if !changed {
		return "skipped", nil
	}
	if dryRun {
		return "would_remove", nil
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(next), mode); err != nil {
		return "skipped", err
	}
	return "removed", nil
}

func upsertBlock(content, block string) (string, bool, error) {
	if err := validateMarkers(content); err != nil {
		return content, false, err
	}
	if oldBefore, oldBlock, ok := extractBlock(content); ok {
		if oldBlock == block {
			return content, false, nil
		}
		// replace old block in-place
		start := strings.Index(content, MarkerBegin)
		end := start + len(oldBlock)
		// extractBlock already includes trailing newline optionally
		_ = oldBefore
		next := content[:start] + block
		if end < len(content) {
			next += content[end:]
		}
		return next, true, nil
	}
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return block, true, nil
	}
	return trimmed + "\n\n" + block, true, nil
}

func stripBlock(content string) (string, bool, error) {
	if err := validateMarkers(content); err != nil {
		return content, false, err
	}
	start := strings.Index(content, MarkerBegin)
	if start < 0 {
		return content, false, nil
	}
	end := strings.Index(content[start:], MarkerEnd)
	if end < 0 {
		return content, false, fmt.Errorf("incomplete easy-proxy-cli block (missing end marker); refusing to modify")
	}
	end = start + end + len(MarkerEnd)
	// swallow trailing newline
	for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
		end++
	}
	out := content[:start] + content[end:]
	out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	return out, true, nil
}

// validateMarkers requires begin/end markers to each appear 0 or exactly once,
// and when both are present begin must precede end.
func validateMarkers(content string) error {
	begins := strings.Count(content, MarkerBegin)
	ends := strings.Count(content, MarkerEnd)
	if begins == 0 && ends == 0 {
		return nil
	}
	if begins == 1 && ends == 1 {
		if strings.Index(content, MarkerBegin) > strings.Index(content, MarkerEnd) {
			return fmt.Errorf("invalid easy-proxy-cli markers (end before begin); refusing to modify")
		}
		return nil
	}
	return fmt.Errorf("invalid easy-proxy-cli markers (begin=%d end=%d); refusing to modify", begins, ends)
}

func extractBlock(content string) (before string, block string, ok bool) {
	start := strings.Index(content, MarkerBegin)
	if start < 0 {
		return "", "", false
	}
	endRel := strings.Index(content[start:], MarkerEnd)
	if endRel < 0 {
		return "", "", false
	}
	end := start + endRel + len(MarkerEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start], content[start:end], true
}

func reloadHints(targets []Target) []string {
	seen := map[string]bool{}
	var hints []string
	for _, t := range targets {
		var h string
		switch t.Shell {
		case shell.Zsh:
			h = "reload: source ~/.zshrc   (or open a new terminal)"
		case shell.Bash:
			h = "reload: source ~/.bashrc  (or open a new terminal)"
		case shell.Fish:
			h = "reload: source ~/.config/fish/config.fish"
		case shell.PowerShell:
			h = "reload: . $PROFILE   (or open a new PowerShell)"
		case shell.Posix:
			h = "reload: . ~/.profile"
		case shell.Nu:
			h = "reload: source $nu.config-path"
		default:
			h = "open a new terminal to load the hook"
		}
		if !seen[h] {
			seen[h] = true
			hints = append(hints, h)
		}
	}
	return hints
}

// KindFromPath guesses shell from filename (for tests / display).
func KindFromPath(path string) shell.Kind {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(base, "zsh"):
		return shell.Zsh
	case strings.Contains(base, "bash"):
		return shell.Bash
	case strings.Contains(base, "fish"):
		return shell.Fish
	case strings.Contains(base, "profile.ps1") || strings.HasSuffix(base, ".ps1"):
		return shell.PowerShell
	case base == "config.nu" || strings.HasSuffix(base, ".nu"):
		return shell.Nu
	case base == ".profile":
		return shell.Posix
	default:
		return shell.Posix
	}
}

// OS returns runtime.GOOS (test seam).
func OS() string { return runtime.GOOS }
