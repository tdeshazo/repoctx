// Package obligation builds a bounded, non-executing handoff of applicable
// repository declarations alongside an existing source-context task.
package obligation

import "github.com/tdeshazo/repoctx/pkg/artifacts"

// Version identifies the obligation handoff wire contract.
const Version = "repoctx.obligations/v1alpha1"

// Options bounds the final handoff. Zero selects a conservative default.
type Options struct {
	MaxArtifacts     int
	MaxRelationships int
	MaxDiagnostics   int
	MaxBytes         int
}

// Bundle binds selected declarations to an already verified context task.
// It intentionally has no command, registration, execution-status, or result
// fields. RunnerCheckID is an opaque identifier for a separate trusted runner.
type Bundle struct {
	Version        string         `json:"version"`
	ID             string         `json:"id"`
	SourceTaskID   string         `json:"source_task_id"`
	SourceSnapshot string         `json:"source_snapshot"`
	Evidence       []string       `json:"evidence"`
	Artifacts      []Artifact     `json:"artifacts"`
	Relationships  []Relationship `json:"relationships"`
	Diagnostics    []Diagnostic   `json:"diagnostics"`
	Omissions      Omissions      `json:"omissions"`
	Capabilities   Capabilities   `json:"capabilities"`
}

// Artifact is a caller-authorized declaration applicable to selected source.
type Artifact struct {
	ID             string                    `json:"id"`
	Kind           string                    `json:"kind"`
	LifecycleClaim string                    `json:"lifecycle_claim"`
	Scopes         []artifacts.Applicability `json:"scopes"`
	Sources        []Source                  `json:"sources"`
	RunnerCheckID  *string                   `json:"runner_check_id,omitempty"`
}

// Source retains declaration provenance and records whether the existing
// context payload contains the exact claimed range.
type Source struct {
	artifacts.Span
	EvidenceID   string `json:"evidence_id,omitempty"`
	Verification string `json:"verification"`
}

// Relationship is a repository-declared claim between selected artifacts.
// It does not assert successful verification or runtime causality.
type Relationship struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	Kind       string   `json:"kind"`
	Resolution string   `json:"resolution"`
	Sources    []Source `json:"sources"`
}

// Diagnostic carries authority-resolution failures without repository prose.
type Diagnostic struct {
	Code                string   `json:"code"`
	ArtifactIDs         []string `json:"artifact_ids"`
	ArtifactIndexes     []int    `json:"artifact_indexes"`
	RelationshipIndexes []int    `json:"relationship_indexes"`
	Path                string   `json:"path,omitempty"`
}

// Omissions reports bounded records that were not emitted.
type Omissions struct {
	Artifacts     int `json:"artifacts"`
	Relationships int `json:"relationships"`
	Diagnostics   int `json:"diagnostics"`
	Budget        int `json:"budget"`
}

// Capabilities makes the execution boundary machine-readable.
type Capabilities struct {
	Available   []string `json:"available"`
	Unavailable []string `json:"unavailable"`
}

// Result contains the bundle and its canonical, newline-terminated JSON.
type Result struct {
	Bundle  *Bundle
	Payload []byte
}
