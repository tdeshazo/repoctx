package compiler

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

func Write(repo *ir.Repository, path string, pretty bool) error {
	if err := repo.Validate(); err != nil {
		return fmt.Errorf("refuse invalid generation: %w", err)
	}
	var w io.Writer = os.Stdout
	var f *os.File
	var gz *gzip.Writer
	if path != "" && path != "-" {
		var err error
		f, err = os.CreateTemp(filepath.Dir(path), ".repoctx-index-*")
		if err != nil {
			return err
		}
		defer f.Close()
		defer os.Remove(f.Name())
		w = f
		if filepath.Ext(path) == ".gz" {
			gz = gzip.NewWriter(f)
			w = gz
		}
	}
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(repo); err != nil {
		return err
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			return err
		}
	}
	if f != nil {
		if err := f.Sync(); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return os.Rename(f.Name(), path)
	}
	return nil
}

func Read(path string) (*ir.Repository, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if filepath.Ext(path) == ".gz" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	var repo ir.Repository
	lim := &io.LimitedReader{R: r, N: (128 << 20) + 1}
	dec := json.NewDecoder(lim)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&repo); err != nil {
		return nil, fmt.Errorf("decode IR: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("IR must contain exactly one JSON object")
	}
	if lim.N <= 0 {
		return nil, fmt.Errorf("IR exceeds 128 MiB decoded limit")
	}
	if err := repo.Validate(); err != nil {
		return nil, fmt.Errorf("invalid IR: %w", err)
	}
	return &repo, nil
}
