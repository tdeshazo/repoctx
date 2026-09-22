package discovery

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// documentMetadata is the small, explicit subset of the parent document
// contract used by bounded discovery. It is deliberately not a second
// authority model: values are only a routing hint for declared navigation and
// document records.
type documentMetadata struct {
	Schema    string `yaml:"schema"`
	Kind      string `yaml:"kind"`
	Origin    string `yaml:"origin"`
	Status    string `yaml:"status"`
	Authority string `yaml:"authority"`
}

// discoveryMetadata reads only a leading YAML frontmatter block. Invalid or
// incomplete metadata is ignored so discovery remains useful for ordinary
// source files and preserves its existing evidence behavior.
func discoveryMetadata(data []byte) (documentMetadata, bool) {
	firstEnd := bytes.IndexByte(data, '\n')
	if firstEnd < 0 || !bytes.Equal(bytes.TrimSpace(data[:firstEnd]), []byte("---")) {
		return documentMetadata{}, false
	}
	start := firstEnd + 1
	end := -1
	for offset := start; offset < len(data); {
		lineEnd := bytes.IndexByte(data[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(data) - offset
		}
		line := data[offset : offset+lineEnd]
		if bytes.Equal(bytes.TrimSpace(line), []byte("---")) || bytes.Equal(bytes.TrimSpace(line), []byte("...")) {
			end = offset
			break
		}
		offset += lineEnd
		if offset < len(data) {
			offset++
		}
	}
	if end < 0 {
		return documentMetadata{}, false
	}
	var metadata documentMetadata
	if err := yaml.Unmarshal(data[start:end], &metadata); err != nil {
		return documentMetadata{}, false
	}
	return metadata, true
}

// metadataRank returns a bounded routing preference from explicit document
// metadata. Canonical active leaves outrank generated navigation maps so a
// small result budget does not get consumed by repeated router windows. A
// generated map is still returned when it is the only matching evidence.
//
// The rank is intentionally internal and omitted from the discovery contract;
// lexical score and exact evidence remain the caller-visible explanation.
func metadataRank(data []byte) int {
	metadata, ok := discoveryMetadata(data)
	if !ok {
		return 0
	}
	if metadata.Schema == "rcx.navigation/v1" &&
		metadata.Origin == "generated" && metadata.Kind == "map" {
		if metadata.Status == "deprecated" {
			return -4
		}
		return -3
	}
	if metadata.Schema != "rcx.document/v1" || metadata.Origin == "" || metadata.Origin == "generated" {
		return 0
	}
	if metadata.Status == "deprecated" {
		return -2
	}
	if metadata.Status == "active" {
		return 3
	}
	return 2
}
