package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tdeshazo/repoctx/pkg/artifacts"
)

func artifactsCmd(args []string) {
	if err := runArtifacts(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "repoctx:", err)
		var usage *artifactsUsageError
		if errors.As(err, &usage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

type artifactsUsageError struct{ message string }

func (e *artifactsUsageError) Error() string { return e.message }

func runArtifacts(args []string) error {
	fs := flag.NewFlagSet("artifacts", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root containing anchored sources")
	source := fs.String("source", "", "artifact authoring JSON")
	out := fs.String("o", "-", "generated artifact catalog or - for stdout")
	check := fs.Bool("check", false, "fail instead of writing when the output is stale")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return &artifactsUsageError{message: err.Error()}
	}
	if fs.NArg() != 0 || *source == "" {
		return &artifactsUsageError{message: "usage: repoctx artifacts -source FILE [-root DIR] [-o FILE] [-check]"}
	}
	if *check && (*out == "" || *out == "-") {
		return &artifactsUsageError{message: "artifacts -check requires a file passed with -o"}
	}
	authoring, err := readBounded(*source, 1<<20)
	if err != nil {
		return fmt.Errorf("read artifact authoring source: %w", err)
	}
	generated, err := artifacts.Generate(*root, authoring, artifacts.Limits{})
	if err != nil {
		return err
	}
	if !*check {
		return writePayload(*out, generated)
	}
	current, err := os.ReadFile(*out)
	if err != nil {
		return fmt.Errorf("read generated artifact catalog: %w", err)
	}
	if !bytes.Equal(current, generated) {
		return fmt.Errorf("generated artifact catalog is stale: run without -check")
	}
	return nil
}

func readBounded(path string, max int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > max {
		return nil, fmt.Errorf("file exceeds byte limit")
	}
	return content, nil
}
