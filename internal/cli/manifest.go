package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tdeshazo/repoctx/pkg/manifest"
)

func manifestCmd(args []string) {
	if err := runManifest(args, os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "repoctx:", err)
		var usage *manifestUsageError
		if errors.As(err, &usage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

type manifestUsageError struct{ message string }

func (e *manifestUsageError) Error() string { return e.message }

func runManifest(args []string, output io.Writer) error {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	root := fs.String("root", ".", "caller-selected repository root")
	file := fs.String("file", "agent-context.yaml", "manifest path")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return &manifestUsageError{message: err.Error()}
	}
	if fs.NArg() != 0 || *file == "" {
		return &manifestUsageError{message: "usage: repoctx manifest [-root DIR] [-file agent-context.yaml]"}
	}
	doc, err := manifest.LoadFile(*root, *file, manifest.Limits{})
	if err != nil {
		return fmt.Errorf("validate repository manifest: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(doc); err != nil {
		return fmt.Errorf("write repository manifest: %w", err)
	}
	return nil
}
