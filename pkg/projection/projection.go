package projection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	defaultMaxBytes         = 32 << 10
	defaultMaxItems         = 256
	defaultMaxEvidenceBytes = 16 << 10
	defaultMaxSourceMaps    = 512
	maximumMaxBytes         = 1 << 20
	maximumMaxItems         = 4096
	maximumMaxEvidenceBytes = 1 << 20
	maximumMaxSourceMaps    = 4096
	maximumTextBytes        = 1 << 20
	maximumAggregateItems   = 8192
	maximumAggregateText    = 8 << 20
)

var allCapabilities = []string{
	CapabilityImperativeCore,
	CapabilityTaskContract,
	CapabilityRepositoryEvidence,
	CapabilityExecutionAffordances,
	CapabilityVerificationObligations,
	CapabilityEscaping,
	CapabilitySourceMaps,
}

// Negotiate validates a caller-approved contract, selects only capabilities
// advertised by target, applies target limits, and returns deterministic JSON.
// It performs no filesystem, network, process, provider, or vendor-file I/O.
func Negotiate(contract Contract, target Target) (Result, error) {
	if err := validateContract(contract); err != nil {
		return Result{}, err
	}
	target, limits, err := normalizeTarget(target)
	if err != nil {
		return Result{}, err
	}
	contract = normalizeContract(contract)

	p := Projection{
		Version: Version,
		Target:  target.ID,
		Escape:  target.Escape,
		Capabilities: Capabilities{
			Available:   append([]string{}, target.Capabilities...),
			Unavailable: unavailableCapabilities(target.Capabilities),
		},
		Core:         []CoreItem{},
		Task:         []TaskItem{},
		Evidence:     []EvidenceItem{},
		Execution:    []ExecutionItem{},
		Verification: []VerificationItem{},
		SourceMaps:   []SourceMap{},
		Omissions:    []Omission{},
	}

	if hasCapability(target, CapabilityImperativeCore) {
		p.Core = boundedCore(contract.Core, limits.MaxItems, &p)
	} else {
		addOmission(&p, LaneImperativeCore, CapabilityImperativeCore, len(contract.Core), "unsupported")
	}
	if hasCapability(target, CapabilityTaskContract) {
		p.Task = boundedTask(contract.Task, limits.MaxItems, &p)
	} else {
		addOmission(&p, LaneTaskContract, CapabilityTaskContract, len(contract.Task), "unsupported")
	}
	if hasCapability(target, CapabilityRepositoryEvidence) {
		p.Evidence = boundedEvidence(contract.Evidence, limits, &p)
	} else {
		addOmission(&p, LaneRepositoryEvidence, CapabilityRepositoryEvidence, len(contract.Evidence), "unsupported")
	}
	if hasCapability(target, CapabilityExecutionAffordances) {
		p.Execution = boundedExecution(contract.Execution, limits.MaxItems, &p)
	} else {
		addOmission(&p, LaneExecutionAffordances, CapabilityExecutionAffordances, len(contract.Execution), "unsupported")
	}
	if hasCapability(target, CapabilityVerificationObligations) {
		p.Verification = boundedVerification(contract.Verification, limits.MaxItems, &p)
	} else {
		addOmission(&p, LaneVerificationObligations, CapabilityVerificationObligations, len(contract.Verification), "unsupported")
	}
	if hasCapability(target, CapabilitySourceMaps) {
		p.SourceMaps = boundedSourceMaps(projectedSourceMaps(contract.SourceMaps, p, &p), limits.MaxSourceMaps, &p)
	} else {
		addOmission(&p, "", CapabilitySourceMaps, len(contract.SourceMaps), "unsupported")
	}
	canonicalizeOmissions(&p)

	payload, err := fitPayload(&p, limits.MaxBytes)
	if err != nil {
		return Result{}, err
	}
	return Result{Projection: p, Payload: payload}, nil
}

func normalizeTarget(target Target) (Target, Limits, error) {
	if !validID(target.ID) {
		return Target{}, Limits{}, invalid("invalid target id")
	}
	seen := map[string]bool{}
	for _, capability := range target.Capabilities {
		if !knownCapability(capability) {
			return Target{}, Limits{}, invalid("unsupported target capability %q", capability)
		}
		if seen[capability] {
			return Target{}, Limits{}, invalid("duplicate target capability %q", capability)
		}
		seen[capability] = true
	}
	target.Capabilities = append([]string{}, target.Capabilities...)
	sort.Strings(target.Capabilities)
	if target.Escape == "" {
		target.Escape = EscapeNone
	}
	if target.Escape != EscapeNone && target.Escape != EscapeJSON {
		return Target{}, Limits{}, invalid("unsupported escape mode %q", target.Escape)
	}
	if target.Escape == EscapeJSON && !seen[CapabilityEscaping] {
		return Target{}, Limits{}, invalid("json escape mode requires %q capability", CapabilityEscaping)
	}
	limits, err := normalizeLimits(target.Limits)
	if err != nil {
		return Target{}, Limits{}, err
	}
	return target, limits, nil
}

func normalizeLimits(limits Limits) (Limits, error) {
	if limits.MaxBytes == 0 {
		limits.MaxBytes = defaultMaxBytes
	}
	if limits.MaxItems == 0 {
		limits.MaxItems = defaultMaxItems
	}
	if limits.MaxEvidenceBytes == 0 {
		limits.MaxEvidenceBytes = defaultMaxEvidenceBytes
	}
	if limits.MaxSourceMaps == 0 {
		limits.MaxSourceMaps = defaultMaxSourceMaps
	}
	if limits.MaxBytes < 1 || limits.MaxBytes > maximumMaxBytes ||
		limits.MaxItems < 1 || limits.MaxItems > maximumMaxItems ||
		limits.MaxEvidenceBytes < 1 || limits.MaxEvidenceBytes > maximumMaxEvidenceBytes ||
		limits.MaxSourceMaps < 1 || limits.MaxSourceMaps > maximumMaxSourceMaps {
		return Limits{}, invalid("target limits exceed bounded projection ceilings")
	}
	return limits, nil
}

func validateContract(contract Contract) error {
	if contract.Version != Version {
		return invalid("unsupported contract version %q", contract.Version)
	}
	if !validID(contract.ID) {
		return invalid("invalid contract id")
	}
	if len(contract.Core) > maximumMaxItems || len(contract.Task) > maximumMaxItems ||
		len(contract.Evidence) > maximumMaxItems || len(contract.Execution) > maximumMaxItems ||
		len(contract.Verification) > maximumMaxItems || len(contract.SourceMaps) > maximumMaxSourceMaps {
		return invalid("contract exceeds bounded item ceilings")
	}
	if err := validateAggregate(contract); err != nil {
		return err
	}
	if err := validateCore(contract.Core); err != nil {
		return err
	}
	if err := validateTask(contract.Task); err != nil {
		return err
	}
	if err := validateEvidence(contract.Evidence); err != nil {
		return err
	}
	if err := validateExecution(contract.Execution); err != nil {
		return err
	}
	if err := validateVerification(contract.Verification); err != nil {
		return err
	}
	if err := validateSourceMaps(contract); err != nil {
		return err
	}
	return nil
}

// validateAggregate runs before normalization copies or sorts any lane. The
// per-item ceilings are intentionally not sufficient on their own: a target
// must not be able to make negotiation allocate or repeatedly encode an
// unbounded collection of individually valid records.
func validateAggregate(contract Contract) error {
	count := len(contract.Core) + len(contract.Task) + len(contract.Evidence) +
		len(contract.Execution) + len(contract.Verification) + len(contract.SourceMaps)
	if count > maximumAggregateItems {
		return invalid("contract exceeds aggregate item ceiling")
	}
	total := 0
	add := func(value string) bool {
		if len(value) > maximumAggregateText-total {
			return false
		}
		total += len(value)
		return true
	}
	for _, item := range contract.Core {
		if !add(item.ID) || !add(item.Kind) || !add(item.Text) {
			return invalid("contract exceeds aggregate text ceiling")
		}
	}
	for _, item := range contract.Task {
		if !add(item.ID) || !add(item.Kind) || !add(item.Text) {
			return invalid("contract exceeds aggregate text ceiling")
		}
	}
	for _, item := range contract.Evidence {
		if !add(item.ID) || !add(item.Kind) || !add(item.Text) || !addSourceText(add, item.Source) {
			return invalid("contract exceeds aggregate text ceiling")
		}
	}
	for _, item := range contract.Execution {
		if !add(item.ID) || !add(item.Kind) || !add(item.Value) {
			return invalid("contract exceeds aggregate text ceiling")
		}
	}
	for _, item := range contract.Verification {
		if !add(item.ID) || !add(item.Kind) || !add(item.Target) || !add(item.Expected) {
			return invalid("contract exceeds aggregate text ceiling")
		}
	}
	for _, item := range contract.SourceMaps {
		if !add(item.ItemID) || !addSourceText(add, item.Source) {
			return invalid("contract exceeds aggregate text ceiling")
		}
	}
	return nil
}

func addSourceText(add func(string) bool, source SourceRef) bool {
	return add(source.ID) && add(source.Path) && add(source.SHA256)
}

func validateCore(items []CoreItem) error {
	seen := map[string]bool{}
	for i, item := range items {
		if err := validateItemID(item.ID, seen); err != nil {
			return invalid("imperative_core[%d]: %v", i, err)
		}
		if err := validateText(item.Kind, maximumTextBytes); err != nil {
			return invalid("imperative_core[%d].kind: %v", i, err)
		}
		if err := validateText(item.Text, maximumTextBytes); err != nil {
			return invalid("imperative_core[%d].text: %v", i, err)
		}
	}
	return nil
}

func validateTask(items []TaskItem) error {
	seen := map[string]bool{}
	for i, item := range items {
		if err := validateItemID(item.ID, seen); err != nil {
			return invalid("task_contract[%d]: %v", i, err)
		}
		if err := validateText(item.Kind, maximumTextBytes); err != nil {
			return invalid("task_contract[%d].kind: %v", i, err)
		}
		if err := validateText(item.Text, maximumTextBytes); err != nil {
			return invalid("task_contract[%d].text: %v", i, err)
		}
	}
	return nil
}

func validateEvidence(items []EvidenceItem) error {
	seen := map[string]bool{}
	for i, item := range items {
		if err := validateItemID(item.ID, seen); err != nil {
			return invalid("repository_evidence[%d]: %v", i, err)
		}
		if err := validateText(item.Kind, maximumTextBytes); err != nil {
			return invalid("repository_evidence[%d].kind: %v", i, err)
		}
		if err := validateText(item.Text, maximumTextBytes); err != nil {
			return invalid("repository_evidence[%d].text: %v", i, err)
		}
		if err := validateSource(item.Source); err != nil {
			return invalid("repository_evidence[%d].source: %v", i, err)
		}
	}
	return nil
}

func validateExecution(items []ExecutionItem) error {
	seen := map[string]bool{}
	for i, item := range items {
		if err := validateItemID(item.ID, seen); err != nil {
			return invalid("execution_affordances[%d]: %v", i, err)
		}
		if err := validateText(item.Kind, maximumTextBytes); err != nil {
			return invalid("execution_affordances[%d].kind: %v", i, err)
		}
		if err := validateText(item.Value, maximumTextBytes); err != nil {
			return invalid("execution_affordances[%d].value: %v", i, err)
		}
	}
	return nil
}

func validateVerification(items []VerificationItem) error {
	seen := map[string]bool{}
	for i, item := range items {
		if err := validateItemID(item.ID, seen); err != nil {
			return invalid("verification_obligations[%d]: %v", i, err)
		}
		for _, field := range []struct {
			name  string
			value string
		}{{"kind", item.Kind}, {"target", item.Target}, {"expected", item.Expected}} {
			if err := validateText(field.value, maximumTextBytes); err != nil {
				return invalid("verification_obligations[%d].%s: %v", i, field.name, err)
			}
		}
	}
	return nil
}

func validateSourceMaps(contract Contract) error {
	seen := map[string]bool{}
	for i, sourceMap := range contract.SourceMaps {
		if !knownLane(sourceMap.Lane) {
			return invalid("source_maps[%d]: unknown lane %q", i, sourceMap.Lane)
		}
		if !validID(sourceMap.ItemID) {
			return invalid("source_maps[%d]: invalid item id", i)
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d", sourceMap.Lane, sourceMap.ItemID, sourceMap.Source.Path, sourceMap.Source.StartByte, sourceMap.Source.EndByte, sourceMap.Source.StartLine, sourceMap.Source.EndLine)
		if seen[key] {
			return invalid("source_maps[%d]: duplicate mapping", i)
		}
		seen[key] = true
		if err := validateSource(sourceMap.Source); err != nil {
			return invalid("source_maps[%d].source: %v", i, err)
		}
		if !itemExists(contract, sourceMap.Lane, sourceMap.ItemID) {
			return invalid("source_maps[%d]: item %q is not declared in lane", i, sourceMap.ItemID)
		}
	}
	return nil
}

func validateSource(source SourceRef) error {
	if source.ID != "" && !validID(source.ID) {
		return fmt.Errorf("invalid source id")
	}
	if !fs.ValidPath(source.Path) || source.Path == "." || strings.ContainsAny(source.Path, "\\\x00:") {
		return fmt.Errorf("invalid source path")
	}
	if source.SHA256 != "" {
		decoded, err := hex.DecodeString(source.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("invalid source digest")
		}
	}
	if source.StartByte < 0 || source.EndByte < source.StartByte || source.StartLine < 1 || source.EndLine < source.StartLine {
		return fmt.Errorf("invalid source span")
	}
	return nil
}

func validateItemID(id string, seen map[string]bool) error {
	if !validID(id) {
		return fmt.Errorf("invalid item id")
	}
	if seen[id] {
		return fmt.Errorf("duplicate item id %q", id)
	}
	seen[id] = true
	return nil
}

func validateText(value string, max int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("invalid UTF-8")
	}
	if len(value) > max {
		return fmt.Errorf("text exceeds %d bytes", max)
	}
	return nil
}

func validID(value string) bool {
	if value == "" || len(value) > 256 || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r <= ' ' || r == '\x7f' || r == '/' || r == '\\' {
			return false
		}
	}
	return true
}

func knownCapability(capability string) bool {
	for _, known := range allCapabilities {
		if capability == known {
			return true
		}
	}
	return false
}

func knownLane(lane Lane) bool {
	return lane == LaneImperativeCore || lane == LaneTaskContract || lane == LaneRepositoryEvidence || lane == LaneExecutionAffordances || lane == LaneVerificationObligations
}

func hasCapability(target Target, capability string) bool {
	for _, candidate := range target.Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

func unavailableCapabilities(capabilities []string) []string {
	result := make([]string, 0, len(allCapabilities))
	for _, capability := range allCapabilities {
		if !hasCapability(Target{Capabilities: capabilities}, capability) {
			result = append(result, capability)
		}
	}
	sort.Strings(result)
	return result
}

func normalizeContract(contract Contract) Contract {
	contract.Core = append([]CoreItem(nil), contract.Core...)
	contract.Task = append([]TaskItem(nil), contract.Task...)
	contract.Evidence = append([]EvidenceItem(nil), contract.Evidence...)
	contract.Execution = append([]ExecutionItem(nil), contract.Execution...)
	contract.Verification = append([]VerificationItem(nil), contract.Verification...)
	contract.SourceMaps = append([]SourceMap(nil), contract.SourceMaps...)
	sort.Slice(contract.Core, func(i, j int) bool { return coreLess(contract.Core[i], contract.Core[j]) })
	sort.Slice(contract.Task, func(i, j int) bool { return taskLess(contract.Task[i], contract.Task[j]) })
	sort.Slice(contract.Evidence, func(i, j int) bool { return evidenceLess(contract.Evidence[i], contract.Evidence[j]) })
	sort.Slice(contract.Execution, func(i, j int) bool { return executionLess(contract.Execution[i], contract.Execution[j]) })
	sort.Slice(contract.Verification, func(i, j int) bool { return verificationLess(contract.Verification[i], contract.Verification[j]) })
	sort.Slice(contract.SourceMaps, func(i, j int) bool { return sourceMapLess(contract.SourceMaps[i], contract.SourceMaps[j]) })
	return contract
}

func coreLess(a, b CoreItem) bool { return tripleLess(a.ID, a.Kind, a.Text, b.ID, b.Kind, b.Text) }
func taskLess(a, b TaskItem) bool { return tripleLess(a.ID, a.Kind, a.Text, b.ID, b.Kind, b.Text) }
func evidenceLess(a, b EvidenceItem) bool {
	return tripleLess(a.ID, a.Kind, a.Text, b.ID, b.Kind, b.Text)
}
func executionLess(a, b ExecutionItem) bool {
	return tripleLess(a.ID, a.Kind, a.Value, b.ID, b.Kind, b.Value)
}
func verificationLess(a, b VerificationItem) bool {
	return quadrupleLess(a.ID, a.Kind, a.Target, a.Expected, b.ID, b.Kind, b.Target, b.Expected)
}
func sourceMapLess(a, b SourceMap) bool {
	if less := tripleLess(string(a.Lane)+"\x00"+a.ItemID, a.Source.Path, a.Source.ID, string(b.Lane)+"\x00"+b.ItemID, b.Source.Path, b.Source.ID); a.Lane != b.Lane || a.ItemID != b.ItemID || a.Source.Path != b.Source.Path || a.Source.ID != b.Source.ID {
		return less
	}
	if a.Source.StartByte != b.Source.StartByte {
		return a.Source.StartByte < b.Source.StartByte
	}
	if a.Source.EndByte != b.Source.EndByte {
		return a.Source.EndByte < b.Source.EndByte
	}
	if a.Source.StartLine != b.Source.StartLine {
		return a.Source.StartLine < b.Source.StartLine
	}
	return a.Source.EndLine < b.Source.EndLine
}
func tripleLess(a1, a2, a3, b1, b2, b3 string) bool {
	if a1 != b1 {
		return a1 < b1
	}
	if a2 != b2 {
		return a2 < b2
	}
	return a3 < b3
}
func quadrupleLess(a1, a2, a3, a4, b1, b2, b3, b4 string) bool {
	if a1 != b1 {
		return a1 < b1
	}
	if a2 != b2 {
		return a2 < b2
	}
	if a3 != b3 {
		return a3 < b3
	}
	return a4 < b4
}

func itemExists(contract Contract, lane Lane, id string) bool {
	switch lane {
	case LaneImperativeCore:
		for _, item := range contract.Core {
			if item.ID == id {
				return true
			}
		}
	case LaneTaskContract:
		for _, item := range contract.Task {
			if item.ID == id {
				return true
			}
		}
	case LaneRepositoryEvidence:
		for _, item := range contract.Evidence {
			if item.ID == id {
				return true
			}
		}
	case LaneExecutionAffordances:
		for _, item := range contract.Execution {
			if item.ID == id {
				return true
			}
		}
	case LaneVerificationObligations:
		for _, item := range contract.Verification {
			if item.ID == id {
				return true
			}
		}
	}
	return false
}

func boundedCore(items []CoreItem, limit int, p *Projection) []CoreItem {
	if len(items) > limit {
		addOmission(p, LaneImperativeCore, CapabilityImperativeCore, len(items)-limit, "item_limit")
		return append([]CoreItem(nil), items[:limit]...)
	}
	return append([]CoreItem(nil), items...)
}
func boundedTask(items []TaskItem, limit int, p *Projection) []TaskItem {
	if len(items) > limit {
		addOmission(p, LaneTaskContract, CapabilityTaskContract, len(items)-limit, "item_limit")
		return append([]TaskItem(nil), items[:limit]...)
	}
	return append([]TaskItem(nil), items...)
}
func boundedEvidence(items []EvidenceItem, limits Limits, p *Projection) []EvidenceItem {
	result := make([]EvidenceItem, 0, min(len(items), limits.MaxItems))
	for _, item := range items {
		if len(result) >= limits.MaxItems {
			addOmission(p, LaneRepositoryEvidence, CapabilityRepositoryEvidence, 1, "item_limit")
			continue
		}
		if len(item.Text) > limits.MaxEvidenceBytes {
			addOmission(p, LaneRepositoryEvidence, CapabilityRepositoryEvidence, 1, "evidence_byte_limit")
			continue
		}
		result = append(result, item)
	}
	return result
}
func boundedExecution(items []ExecutionItem, limit int, p *Projection) []ExecutionItem {
	if len(items) > limit {
		addOmission(p, LaneExecutionAffordances, CapabilityExecutionAffordances, len(items)-limit, "item_limit")
		return append([]ExecutionItem(nil), items[:limit]...)
	}
	return append([]ExecutionItem(nil), items...)
}
func boundedVerification(items []VerificationItem, limit int, p *Projection) []VerificationItem {
	if len(items) > limit {
		addOmission(p, LaneVerificationObligations, CapabilityVerificationObligations, len(items)-limit, "item_limit")
		return append([]VerificationItem(nil), items[:limit]...)
	}
	return append([]VerificationItem(nil), items...)
}
func boundedSourceMaps(items []SourceMap, limit int, p *Projection) []SourceMap {
	result := append([]SourceMap(nil), items...)
	if len(result) > limit {
		addOmission(p, "", CapabilitySourceMaps, len(result)-limit, "source_map_limit")
		result = result[:limit]
	}
	return result
}

func projectedSourceMaps(items []SourceMap, p Projection, result *Projection) []SourceMap {
	selected := make([]SourceMap, 0, len(items))
	for _, item := range items {
		if !projectedItemExists(p, item.Lane, item.ItemID) {
			addOmission(result, item.Lane, CapabilitySourceMaps, 1, "item_not_projected")
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

func projectedItemExists(p Projection, lane Lane, id string) bool {
	switch lane {
	case LaneImperativeCore:
		for _, item := range p.Core {
			if item.ID == id {
				return true
			}
		}
	case LaneTaskContract:
		for _, item := range p.Task {
			if item.ID == id {
				return true
			}
		}
	case LaneRepositoryEvidence:
		for _, item := range p.Evidence {
			if item.ID == id {
				return true
			}
		}
	case LaneExecutionAffordances:
		for _, item := range p.Execution {
			if item.ID == id {
				return true
			}
		}
	case LaneVerificationObligations:
		for _, item := range p.Verification {
			if item.ID == id {
				return true
			}
		}
	}
	return false
}

func addOmission(p *Projection, lane Lane, feature string, count int, reason string) {
	if count <= 0 {
		return
	}
	for i := range p.Omissions {
		if p.Omissions[i].Lane == lane && p.Omissions[i].Feature == feature && p.Omissions[i].Reason == reason {
			p.Omissions[i].Count += count
			p.Incomplete = true
			return
		}
	}
	p.Omissions = append(p.Omissions, Omission{Lane: lane, Feature: feature, Count: count, Reason: reason})
	p.Incomplete = true
}

func canonicalizeOmissions(p *Projection) {
	sort.Slice(p.Omissions, func(i, j int) bool {
		if p.Omissions[i].Lane != p.Omissions[j].Lane {
			return p.Omissions[i].Lane < p.Omissions[j].Lane
		}
		if p.Omissions[i].Feature != p.Omissions[j].Feature {
			return p.Omissions[i].Feature < p.Omissions[j].Feature
		}
		return p.Omissions[i].Reason < p.Omissions[j].Reason
	})
}

func fitPayload(p *Projection, maxBytes int) ([]byte, error) {
	for {
		canonicalizeOmissions(p)
		payload, err := marshalProjection(p)
		if err != nil {
			return nil, err
		}
		if len(payload) <= maxBytes {
			return payload, nil
		}
		lane, count, ok := findByteDrop(p, maxBytes)
		if !ok {
			return nil, invalid("max-bytes %d cannot fit projection metadata", maxBytes)
		}
		dropLane(p, lane, count)
		addByteOmission(p, lane, count)
	}
}

// findByteDrop chooses the first lower-priority lane that can make the
// payload fit, using a binary search over that lane's tail. Each candidate is
// encoded at most O(log n) times rather than once per removed record.
func findByteDrop(p *Projection, maxBytes int) (Lane, int, bool) {
	for _, lane := range []Lane{LaneVerificationObligations, LaneExecutionAffordances, LaneRepositoryEvidence, LaneTaskContract, LaneImperativeCore, ""} {
		length := laneLength(*p, lane)
		if length == 0 {
			continue
		}
		if fitsAfterDrop(p, lane, length, maxBytes) {
			return lane, firstFittingDrop(p, lane, length, maxBytes), true
		}
		// This lane cannot fit by itself; consume it before considering the
		// next, higher-priority lane.
		return lane, length, true
	}
	return "", 0, false
}

func firstFittingDrop(p *Projection, lane Lane, length, maxBytes int) int {
	lo, hi := 1, length
	for lo < hi {
		mid := lo + (hi-lo)/2
		if fitsAfterDrop(p, lane, mid, maxBytes) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func fitsAfterDrop(p *Projection, lane Lane, count, maxBytes int) bool {
	trial := *p
	trial.Omissions = append([]Omission(nil), p.Omissions...)
	dropLane(&trial, lane, count)
	addByteOmission(&trial, lane, count)
	canonicalizeOmissions(&trial)
	payload, err := marshalProjection(&trial)
	return err == nil && len(payload) <= maxBytes
}

func laneLength(p Projection, lane Lane) int {
	switch lane {
	case "":
		return len(p.SourceMaps)
	case LaneVerificationObligations:
		return len(p.Verification)
	case LaneExecutionAffordances:
		return len(p.Execution)
	case LaneRepositoryEvidence:
		return len(p.Evidence)
	case LaneTaskContract:
		return len(p.Task)
	case LaneImperativeCore:
		return len(p.Core)
	default:
		return 0
	}
}

func dropLane(p *Projection, lane Lane, count int) {
	length := laneLength(*p, lane)
	if count < 1 || count > length {
		return
	}
	keep := length - count
	switch lane {
	case "":
		p.SourceMaps = p.SourceMaps[:keep]
	case LaneVerificationObligations:
		p.Verification = p.Verification[:keep]
	case LaneExecutionAffordances:
		p.Execution = p.Execution[:keep]
	case LaneRepositoryEvidence:
		p.Evidence = p.Evidence[:keep]
	case LaneTaskContract:
		p.Task = p.Task[:keep]
	case LaneImperativeCore:
		p.Core = p.Core[:keep]
	}
}

func addByteOmission(p *Projection, lane Lane, count int) {
	if lane == "" {
		addOmission(p, "", CapabilitySourceMaps, count, "byte_limit")
		return
	}
	addOmission(p, lane, string(lane), count, "byte_limit")
}

func marshalProjection(p *Projection) ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Marshal returns canonical newline-terminated JSON for a projection that has
// already been negotiated. Callers should prefer Result.Payload from
// Negotiate because it includes target limits and omissions.
func Marshal(p Projection) ([]byte, error) {
	if p.Version != Version {
		return nil, invalid("unsupported projection version %q", p.Version)
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Digest identifies the exact canonical payload bytes. It is a content ID,
// not a signature or authority assertion.
func Digest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
