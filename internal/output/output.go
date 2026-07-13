package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Mode controls machine vs human presentation.
type Mode struct {
	JSON bool
}

// Writer separates stdout (data / eval script) from stderr (human hints).
type Writer struct {
	Out  io.Writer
	Err  io.Writer
	Mode Mode
}

func New(mode Mode) Writer {
	return Writer{Out: os.Stdout, Err: os.Stderr, Mode: mode}
}

func (w Writer) JSON(v any) error {
	enc := json.NewEncoder(w.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (w Writer) Human(format string, args ...any) {
	fmt.Fprintf(w.Err, format+"\n", args...)
}

func (w Writer) Script(s string) {
	fmt.Fprint(w.Out, s)
}

// Exit codes — stable for agents.
const (
	ExitOK          = 0
	ExitError       = 1
	ExitMisconfig   = 2
	ExitUnavailable = 3
)
