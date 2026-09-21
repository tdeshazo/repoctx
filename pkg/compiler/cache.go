package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

const (
	parseFragmentVersion = "repoctx.parse-fragment/v1alpha1"
	maxFragmentBytes     = 128 << 20
)

// CacheStats describes file-local parse-fragment reuse for one compilation.
// ReferencedBytes counts unique cache entries used by the resulting generation.
type CacheStats struct {
	Hits            int
	Misses          int
	Invalid         int
	ReadBytes       int64
	WrittenBytes    int64
	ReferencedBytes int64
}

type parseFragment struct {
	Version     string          `json:"version"`
	SourceSHA   string          `json:"source_sha256"`
	Language    ir.Language     `json:"language"`
	File        ir.File         `json:"file"`
	Symbols     []ir.Symbol     `json:"symbols"`
	Edges       []ir.Edge       `json:"edges"`
	Diagnostics []ir.Diagnostic `json:"diagnostics"`
	Strings     []string        `json:"strings"`
}

type fragmentRecord struct {
	Version       string        `json:"version"`
	Key           string        `json:"key"`
	PayloadSHA256 string        `json:"payload_sha256"`
	ParseFragment parseFragment `json:"fragment"`
}

type parseCache struct {
	dir        string
	profileID  string
	memory     map[string]parseFragment
	referenced map[string]bool
	stats      CacheStats
}

func newParseCache(root, directory, profileID string) (*parseCache, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("parse cache path: %w", err)
	}
	if inside(root, abs) {
		return nil, fmt.Errorf("parse cache must be outside the indexed repository")
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("create parse cache: %w", err)
	}
	if err := validateCacheDirectory(abs); err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	realCache, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve parse cache: %w", err)
	}
	if inside(realRoot, realCache) {
		return nil, fmt.Errorf("parse cache must be outside the indexed repository")
	}
	fragments := filepath.Join(abs, "fragments")
	if err := os.MkdirAll(fragments, 0o700); err != nil {
		return nil, fmt.Errorf("create fragment cache: %w", err)
	}
	if err := validateCacheDirectory(fragments); err != nil {
		return nil, err
	}
	return &parseCache{dir: fragments, profileID: profileID,
		memory: map[string]parseFragment{}, referenced: map[string]bool{}}, nil
}

func inside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && (rel == "." || rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func validateCacheDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect parse cache directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("parse cache path must be a real directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("parse cache directory must not be group- or world-writable")
	}
	return nil
}

func (c *parseCache) key(language ir.Language, sourceSHA string) (string, error) {
	id, err := ir.ContentID(struct {
		Version   string
		ProfileID string
		Language  ir.Language
		SourceSHA string
	}{parseFragmentVersion, c.profileID, language, sourceSHA})
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(id, "sha256:"), nil
}

func (c *parseCache) path(key string) string {
	return filepath.Join(c.dir, key[:2], key[2:]+".json")
}

func (c *parseCache) load(language ir.Language, sourceSHA string) (parseFragment, bool, error) {
	key, err := c.key(language, sourceSHA)
	if err != nil {
		return parseFragment{}, false, err
	}
	if fragment, ok := c.memory[key]; ok {
		c.stats.Hits++
		return fragment, true, nil
	}
	path := c.path(key)
	directory := filepath.Dir(path)
	if _, err := os.Lstat(directory); err == nil {
		if err := validateCacheDirectory(directory); err != nil {
			c.stats.Invalid++
			c.stats.Misses++
			return parseFragment{}, false, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return parseFragment{}, false, fmt.Errorf("inspect parse fragment directory: %w", err)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		c.stats.Misses++
		return parseFragment{}, false, nil
	}
	if err != nil {
		return parseFragment{}, false, fmt.Errorf("inspect parse fragment: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() < 1 || info.Size() > maxFragmentBytes ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0o022 != 0 {
		c.stats.Invalid++
		c.stats.Misses++
		return parseFragment{}, false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return parseFragment{}, false, fmt.Errorf("open parse fragment: %w", err)
	}
	defer file.Close()
	reader := &io.LimitedReader{R: file, N: maxFragmentBytes + 1}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var record fragmentRecord
	if err := decoder.Decode(&record); err != nil {
		c.stats.Invalid++
		c.stats.Misses++
		return parseFragment{}, false, nil
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || reader.N <= 0 ||
		record.Version != parseFragmentVersion || record.Key != key {
		c.stats.Invalid++
		c.stats.Misses++
		return parseFragment{}, false, nil
	}
	payload, err := json.Marshal(record.ParseFragment)
	if err != nil || fullHash(payload) != record.PayloadSHA256 ||
		validateParseFragment(record.ParseFragment, language, sourceSHA) != nil {
		c.stats.Invalid++
		c.stats.Misses++
		return parseFragment{}, false, nil
	}
	c.memory[key] = record.ParseFragment
	c.stats.Hits++
	c.stats.ReadBytes += info.Size()
	c.reference(key, info.Size())
	return record.ParseFragment, true, nil
}

func (c *parseCache) store(fragment parseFragment) error {
	key, err := c.key(fragment.Language, fragment.SourceSHA)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(fragment)
	if err != nil {
		return fmt.Errorf("encode parse fragment payload: %w", err)
	}
	record := fragmentRecord{Version: parseFragmentVersion, Key: key,
		PayloadSHA256: fullHash(payload), ParseFragment: fragment}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode parse fragment: %w", err)
	}
	encoded = append(encoded, '\n')
	if len(encoded) > maxFragmentBytes {
		return fmt.Errorf("parse fragment exceeds %d-byte cache limit", maxFragmentBytes)
	}
	path := c.path(key)
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create parse fragment directory: %w", err)
	}
	if err := validateCacheDirectory(directory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".fragment-*")
	if err != nil {
		return fmt.Errorf("create parse fragment: %w", err)
	}
	tempPath := temporary.Name()
	defer os.Remove(tempPath)
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return fmt.Errorf("write parse fragment: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync parse fragment: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close parse fragment: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publish parse fragment: %w", err)
	}
	c.memory[key] = fragment
	c.stats.WrittenBytes += int64(len(encoded))
	c.reference(key, int64(len(encoded)))
	return nil
}

func (c *parseCache) reference(key string, size int64) {
	if c.referenced[key] {
		return
	}
	c.referenced[key] = true
	c.stats.ReferencedBytes += size
}

func validateParseFragment(fragment parseFragment, language ir.Language, sourceSHA string) error {
	if fragment.Version != parseFragmentVersion || fragment.Language != language ||
		fragment.SourceSHA != sourceSHA || fragment.File.Path != 0 ||
		fragment.File.Lang != language || fragment.File.Hash != sourceSHA {
		return fmt.Errorf("parse fragment identity mismatch")
	}
	validString := func(ref int) bool { return ref >= 0 && ref <= len(fragment.Strings) }
	if !validString(fragment.File.Unit) {
		return fmt.Errorf("invalid fragment unit")
	}
	for _, root := range fragment.File.Roots {
		if root < 0 || root >= len(fragment.File.Nodes) {
			return fmt.Errorf("invalid fragment root")
		}
	}
	for _, node := range fragment.File.Nodes {
		if !validString(node.Kind) || !validString(node.Text) {
			return fmt.Errorf("invalid fragment node string")
		}
		for _, child := range node.Children {
			if child < 0 || child >= len(fragment.File.Nodes) {
				return fmt.Errorf("invalid fragment child")
			}
		}
	}
	for _, symbol := range fragment.Symbols {
		if symbol.ID != 0 || symbol.File != 0 || symbol.Node < 0 ||
			symbol.Node >= len(fragment.File.Nodes) || !validString(symbol.Name) ||
			!validString(symbol.Receiver) || symbol.Parent < 0 ||
			symbol.Parent > len(fragment.Symbols) {
			return fmt.Errorf("invalid fragment symbol")
		}
	}
	for _, edge := range fragment.Edges {
		if edge.From.File != 0 || edge.From.Node < 0 ||
			edge.From.Node >= len(fragment.File.Nodes) || edge.OwnerSymbol != 0 ||
			edge.ToSymbol != 0 || !validString(edge.Text) {
			return fmt.Errorf("invalid fragment edge")
		}
	}
	for _, diagnostic := range fragment.Diagnostics {
		if diagnostic.File != 1 || diagnostic.Message == 0 ||
			!validString(diagnostic.Message) || diagnostic.Severity < ir.SeverityInfo ||
			diagnostic.Severity > ir.SeverityError {
			return fmt.Errorf("invalid fragment diagnostic")
		}
	}
	return nil
}
