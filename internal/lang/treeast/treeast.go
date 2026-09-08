// Package treeast lowers the maintained Tree-sitter grammars used by repoctx
// into the repository's bounded, source-linked AST and conservative symbol
// layers. It deliberately does not attempt type checking, execution, or full
// language semantics.
package treeast

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	treemarkdown "github.com/tree-sitter-grammars/tree-sitter-markdown/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
	treecss "github.com/tree-sitter/tree-sitter-css/bindings/go"
	treehtml "github.com/tree-sitter/tree-sitter-html/bindings/go"
	treejs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	treepython "github.com/tree-sitter/tree-sitter-python/bindings/go"
	treetypescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

// Language identifies one of the bundled Tree-sitter grammars.
type Language uint8

const (
	Python Language = iota + 1
	HTML
	CSS
	JavaScript
	TypeScript
	TSX
	Markdown
)

const (
	maxNodes     = 250_000
	maxDepth     = 4096
	maxTextSize  = 120
	parseTimeout = 20 * time.Second
)

type Result struct {
	Nodes   []ir.Node
	Roots   []int
	Symbols []LocalSymbol
	Edges   []LocalEdge
}

type LocalSymbol struct {
	Name   string
	Kind   ir.SymbolKind
	Node   int
	Parent int // local symbol index + 1; zero means none
}

type LocalEdge struct {
	Kind ir.EdgeKind
	Node int
	Text string
}

// Parse parses source with the selected grammar and lowers named syntax nodes.
// Tree-sitter's error-recovery trees are intentionally rejected as a whole:
// callers receive a bounded diagnostic instead of a partial semantic claim.
func Parse(language Language, src []byte, st *ir.Strings) (Result, error) {
	if !utf8.Valid(src) {
		return Result{}, fmt.Errorf("%s tree-sitter input is not UTF-8", languageName(language))
	}
	grammar, ok := grammarFor(language)
	if !ok {
		return Result{}, fmt.Errorf("unsupported tree-sitter language %d", language)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(grammar); err != nil {
		return Result{}, fmt.Errorf("set %s grammar: %w", languageName(language), err)
	}
	parser.SetTimeoutMicros(uint64(parseTimeout / time.Microsecond))
	tree := parser.Parse(src, nil)
	if tree == nil {
		return Result{}, fmt.Errorf("%s tree-sitter parser returned no tree (possibly timed out)", languageName(language))
	}
	defer tree.Close()
	root := tree.RootNode()
	b := builder{language: language, src: src, st: st, nodes: make([]ir.Node, 0, 256), nodeIndex: map[uintptr]int{}}
	rootIndex, err := b.add(root, 0)
	if err != nil {
		return Result{}, err
	}
	if language == Markdown {
		b.collectMarkdownReferenceDefinitions(root)
		if err := b.attachMarkdownInline(root); err != nil {
			return Result{}, err
		}
	}
	if root.HasError() {
		return Result{}, fmt.Errorf("%s tree-sitter syntax errors (root=%s)", languageName(language), root.Kind())
	}
	b.collect(root, 0)
	return Result{Nodes: b.nodes, Roots: []int{rootIndex}, Symbols: b.symbols, Edges: b.edges}, nil
}

func grammarFor(language Language) (*sitter.Language, bool) {
	switch language {
	case Python:
		return sitter.NewLanguage(treepython.Language()), true
	case HTML:
		return sitter.NewLanguage(treehtml.Language()), true
	case CSS:
		return sitter.NewLanguage(treecss.Language()), true
	case JavaScript:
		return sitter.NewLanguage(treejs.Language()), true
	case TypeScript:
		return sitter.NewLanguage(treetypescript.LanguageTypescript()), true
	case TSX:
		return sitter.NewLanguage(treetypescript.LanguageTSX()), true
	case Markdown:
		return sitter.NewLanguage(treemarkdown.Language()), true
	default:
		return nil, false
	}
}

func languageName(language Language) string {
	switch language {
	case Python:
		return "python"
	case HTML:
		return "html"
	case CSS:
		return "css"
	case JavaScript:
		return "javascript"
	case TypeScript:
		return "typescript"
	case TSX:
		return "tsx"
	case Markdown:
		return "markdown"
	default:
		return "unknown"
	}
}

type builder struct {
	language           Language
	src                []byte
	st                 *ir.Strings
	nodes              []ir.Node
	nodeIndex          map[uintptr]int
	markdownReferences map[string]string
	symbols            []LocalSymbol
	edges              []LocalEdge
}

func (b *builder) add(n *sitter.Node, depth int) (int, error) {
	if n == nil {
		return -1, nil
	}
	if len(b.nodes) >= maxNodes {
		return -1, fmt.Errorf("%s tree-sitter AST exceeds %d nodes", languageName(b.language), maxNodes)
	}
	if depth > maxDepth {
		return -1, fmt.Errorf("%s tree-sitter AST exceeds depth bound", languageName(b.language))
	}
	idx := len(b.nodes)
	b.nodeIndex[n.Id()] = idx
	b.nodes = append(b.nodes, ir.Node{Kind: b.st.Intern(n.Kind()), Text: b.st.Intern(b.text(n)), Span: b.span(n)})
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		ci, err := b.add(child, depth+1)
		if err != nil {
			return -1, err
		}
		if ci >= 0 {
			b.nodes[idx].Children = append(b.nodes[idx].Children, ci)
		}
	}
	return idx, nil
}

func span(n *sitter.Node) ir.Span {
	start, end := n.StartPosition(), n.EndPosition()
	return ir.Span{SL: int(start.Row) + 1, SC: int(start.Column), EL: int(end.Row) + 1, EC: int(end.Column)}
}

func (b *builder) span(n *sitter.Node) ir.Span {
	sp := span(n)
	// Keep the compatibility behavior of the former Python AST front end:
	// definitions selected as symbols include decorators in their source span.
	if b.language == Python && (n.Kind() == "function_definition" || n.Kind() == "async_function_definition" || n.Kind() == "class_definition") {
		if parent := n.Parent(); parent != nil && parent.Kind() == "decorated_definition" {
			sp.SL, sp.SC = int(parent.StartPosition().Row)+1, int(parent.StartPosition().Column)
		}
	}
	return sp
}

func (b *builder) text(n *sitter.Node) string {
	kind := n.Kind()
	if !textKind(kind) {
		return ""
	}
	text := n.Utf8Text(b.src)
	if len(text) == 0 || len(text) > maxTextSize || strings.ContainsAny(text, "\r\n") {
		return ""
	}
	return strings.TrimSpace(text)
}

func textKind(kind string) bool {
	if strings.Contains(kind, "identifier") || strings.HasSuffix(kind, "_name") {
		return true
	}
	switch kind {
	case "class_name", "id_name", "tag_name", "property_name", "attribute_name", "string", "string_literal", "integer", "float", "number":
		return true
	default:
		return false
	}
}

func (b *builder) collect(n *sitter.Node, parent int) {
	idx := b.nodeIndex[n.Id()]
	nextParent := parent
	if decl, ok := b.declaration(n, parent); ok {
		b.symbols = append(b.symbols, decl)
		nextParent = len(b.symbols) // local index + 1
	}
	for _, edge := range b.edgesFor(n, idx) {
		b.edges = append(b.edges, edge)
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		b.collect(n.NamedChild(i), nextParent)
	}
}

// attachMarkdownInline parses only the ranges explicitly marked inline by the
// Markdown block grammar. In particular, fenced code contents have no inline
// marker and are never sent to the inline parser (or any language parser).
func (b *builder) attachMarkdownInline(n *sitter.Node) error {
	if n == nil {
		return nil
	}
	type inlineRange struct {
		start, end           uint
		startPoint, endPoint sitter.Point
		node                 int
	}
	var inlineNodes []inlineRange
	var collect func(*sitter.Node)
	collect = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "inline" {
			idx, ok := b.nodeIndex[node.Id()]
			if !ok {
				return
			}
			inlineNodes = append(inlineNodes, inlineRange{start: node.StartByte(), end: node.EndByte(), startPoint: node.StartPosition(), endPoint: node.EndPosition(), node: idx})
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			collect(node.NamedChild(i))
		}
	}
	collect(n)
	if len(inlineNodes) == 0 {
		return nil
	}
	ranges := make([]sitter.Range, 0, len(inlineNodes))
	for _, inline := range inlineNodes {
		// The block tree already guarantees source positions for inline nodes.
		ranges = append(ranges, sitter.Range{StartByte: inline.start, EndByte: inline.end, StartPoint: inline.startPoint, EndPoint: inline.endPoint})
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(sitter.NewLanguage(treemarkdown.InlineLanguage())); err != nil {
		return fmt.Errorf("set markdown inline grammar: %w", err)
	}
	parser.SetTimeoutMicros(uint64(parseTimeout / time.Microsecond))
	if err := parser.SetIncludedRanges(ranges); err != nil {
		return fmt.Errorf("set markdown inline ranges: %w", err)
	}
	tree := parser.Parse(b.src, nil)
	if tree == nil {
		return fmt.Errorf("markdown inline parser returned no tree (possibly timed out)")
	}
	defer tree.Close()
	inlineRoot := tree.RootNode()
	if inlineRoot.HasError() {
		return fmt.Errorf("markdown inline tree-sitter syntax errors")
	}
	for i := uint(0); i < inlineRoot.NamedChildCount(); i++ {
		child := inlineRoot.NamedChild(i)
		owner := -1
		for _, inline := range inlineNodes {
			if child.StartByte() >= inline.start && child.EndByte() <= inline.end {
				owner = inline.node
				break
			}
		}
		if owner < 0 {
			continue
		}
		ci, err := b.add(child, 0)
		if err != nil {
			return err
		}
		if ci >= 0 {
			b.nodes[owner].Children = append(b.nodes[owner].Children, ci)
			b.appendMarkdownInlineEdges(child)
		}
	}
	return nil
}

func (b *builder) appendMarkdownInlineEdges(n *sitter.Node) {
	if n == nil {
		return
	}
	if idx, ok := b.nodeIndex[n.Id()]; ok {
		b.edges = append(b.edges, b.edgesFor(n, idx)...)
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		b.appendMarkdownInlineEdges(n.NamedChild(i))
	}
}

type declaration struct {
	name string
	kind ir.SymbolKind
}

func (b *builder) declaration(n *sitter.Node, parent int) (LocalSymbol, bool) {
	var d declaration
	kind := n.Kind()
	switch b.language {
	case Python:
		switch kind {
		case "class_definition":
			d = declaration{b.fieldText(n, "name"), ir.SymType}
		case "function_definition", "async_function_definition":
			d = declaration{b.fieldText(n, "name"), ir.SymFunction}
			if parent > 0 && b.symbols[parent-1].Kind == ir.SymType {
				d.kind = ir.SymMethod
			}
		case "assignment", "named_expression", "type_alias_statement":
			d.name = b.bindingName(n)
			d.kind = ir.SymVariable
		}
	case JavaScript:
		switch kind {
		case "class_declaration":
			d = declaration{b.fieldText(n, "name"), ir.SymType}
		case "function_declaration", "generator_function_declaration":
			d = declaration{b.fieldText(n, "name"), ir.SymFunction}
		case "method_definition":
			d = declaration{b.fieldText(n, "name"), ir.SymMethod}
		case "variable_declarator":
			d = declaration{b.fieldText(n, "name"), ir.SymVariable}
		}
	case TypeScript, TSX:
		switch kind {
		case "class_declaration", "abstract_class_declaration", "interface_declaration", "enum_declaration", "type_alias_declaration", "internal_module":
			d = declaration{b.fieldText(n, "name"), ir.SymType}
		case "function_declaration", "generator_function_declaration", "function_signature":
			d = declaration{b.fieldText(n, "name"), ir.SymFunction}
		case "method_definition", "method_signature", "abstract_method_signature":
			d = declaration{b.fieldText(n, "name"), ir.SymMethod}
		case "variable_declarator":
			d = declaration{b.fieldText(n, "name"), ir.SymVariable}
		}
	case HTML:
		if kind == "element" {
			if name := b.descendantText(n, "tag_name"); name != "" {
				d = declaration{name, ir.SymType}
			}
		}
	case CSS:
		switch kind {
		case "class_selector":
			d = declaration{b.descendantText(n, "class_name"), ir.SymVariable}
		case "id_selector":
			d = declaration{b.descendantText(n, "id_name"), ir.SymVariable}
		case "tag_name":
			d = declaration{b.nodeText(n), ir.SymVariable}
		case "declaration":
			d = declaration{b.descendantText(n, "property_name"), ir.SymVariable}
		}
	case Markdown:
		if kind == "atx_heading" || kind == "setext_heading" {
			d = declaration{b.markdownHeadingText(n), ir.SymModule}
		}
	}
	if d.name == "" {
		return LocalSymbol{}, false
	}
	return LocalSymbol{Name: cleanName(d.name), Kind: d.kind, Node: b.nodeIndex[n.Id()], Parent: parent}, true
}

func (b *builder) markdownHeadingText(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	inline := n.ChildByFieldName("heading_content")
	if inline == nil {
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if child := n.NamedChild(i); child.Kind() == "inline" {
				inline = child
				break
			}
		}
	}
	text := b.nodeText(inline)
	if text == "" {
		return ""
	}
	// Keep heading labels readable while retaining source-linked spans. This is
	// intentionally conservative; it does not interpret arbitrary Markdown.
	for _, marker := range []string{"**", "__", "~~", "`", "*"} {
		text = strings.ReplaceAll(text, marker, "")
	}
	return strings.Join(strings.Fields(text), " ")
}

func (b *builder) fieldText(n *sitter.Node, field string) string {
	child := n.ChildByFieldName(field)
	if child == nil {
		return ""
	}
	return b.nodeText(child)
}

func (b *builder) descendantText(n *sitter.Node, kind string) string {
	if n.Kind() == kind {
		return b.nodeText(n)
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if text := b.descendantText(n.NamedChild(i), kind); text != "" {
			return text
		}
	}
	return ""
}

func (b *builder) nodeText(n *sitter.Node) string {
	if n == nil || n.EndByte() < n.StartByte() {
		return ""
	}
	a, z := int(n.StartByte()), int(n.EndByte())
	if a < 0 || z > len(b.src) || z-a == 0 || z-a > maxTextSize {
		return ""
	}
	text := strings.TrimSpace(string(b.src[a:z]))
	if strings.ContainsAny(text, "\r\n") || !utf8.ValidString(text) {
		return ""
	}
	return text
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "'\"")
	name = strings.TrimLeft(name, ".#")
	return name
}

func (b *builder) bindingName(n *sitter.Node) string {
	left := n.ChildByFieldName("left")
	if left == nil {
		left = n.ChildByFieldName("name")
	}
	if left == nil {
		return ""
	}
	return b.bindingNameNode(left)
}

func (b *builder) bindingNameNode(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case "identifier", "type_identifier", "property_identifier", "private_property_identifier":
		return b.nodeText(n)
	case "attribute", "member_expression", "subscript", "assignment_pattern":
		return ""
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if name := b.bindingNameNode(n.NamedChild(i)); name != "" {
			return name
		}
	}
	return ""
}

func (b *builder) edgesFor(n *sitter.Node, idx int) []LocalEdge {
	var out []LocalEdge
	switch b.language {
	case Python:
		switch n.Kind() {
		case "import_statement", "import_from_statement":
			for _, text := range b.importTexts(n) {
				out = append(out, LocalEdge{Kind: ir.EdgeImports, Node: idx, Text: text})
			}
		case "call":
			if text := b.expressionName(n.ChildByFieldName("function")); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeCalls, Node: idx, Text: text})
			}
		}
	case JavaScript:
		switch n.Kind() {
		case "import_statement":
			if text := cleanName(b.fieldText(n, "source")); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeImports, Node: idx, Text: text})
			}
		case "call_expression", "new_expression":
			if text := b.expressionName(n.ChildByFieldName("function")); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeCalls, Node: idx, Text: text})
			} else if text := b.expressionName(n.ChildByFieldName("constructor")); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeCalls, Node: idx, Text: text})
			}
		}
	case TypeScript, TSX:
		switch n.Kind() {
		case "import_statement":
			if text := importSourceText(b.typeScriptImportSource(n)); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeImports, Node: idx, Text: text})
			}
		case "call_expression", "new_expression":
			if text := b.expressionName(n.ChildByFieldName("function")); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeCalls, Node: idx, Text: text})
			} else if text := b.expressionName(n.ChildByFieldName("constructor")); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeCalls, Node: idx, Text: text})
			}
		}
	case CSS:
		if n.Kind() == "import_statement" {
			if text := cleanName(b.nodeText(n)); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeImports, Node: idx, Text: text})
			}
		}
	case HTML:
		if n.Kind() == "attribute" {
			var name, value string
			for i := uint(0); i < n.NamedChildCount(); i++ {
				child := n.NamedChild(i)
				switch child.Kind() {
				case "attribute_name":
					name = strings.ToLower(b.nodeText(child))
				case "attribute_value", "quoted_attribute_value":
					value = cleanName(b.nodeText(child))
				}
			}
			if (name == "src" || name == "href") && value != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeImports, Node: idx, Text: value})
			}
		}
	case Markdown:
		switch n.Kind() {
		case "inline_link", "full_reference_link", "collapsed_reference_link", "shortcut_link", "image", "uri_autolink", "email_autolink":
			if text := b.markdownDestination(n); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeReferences, Node: idx, Text: text})
			}
		case "link_reference_definition":
			if text := b.markdownDestination(n); text != "" {
				out = append(out, LocalEdge{Kind: ir.EdgeReferences, Node: idx, Text: text})
			}
		}
	}
	return out
}

func (b *builder) markdownDestination(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child.Kind() == "link_destination" || child.Kind() == "uri" || child.Kind() == "email" {
			if text := markdownText(b.nodeText(child)); text != "" {
				return text
			}
		}
		if text := b.markdownDestination(child); text != "" {
			return text
		}
	}
	if label := b.markdownReferenceLabel(n); label != "" {
		return b.markdownReferences[label]
	}
	// Autolinks have no named destination in the maintained grammar; their
	// bounded source text is still useful as a syntactic reference.
	if n.Kind() == "uri_autolink" || n.Kind() == "email_autolink" {
		return markdownText(b.nodeText(n))
	}
	return ""
}

// collectMarkdownReferenceDefinitions records the first destination for each
// normalized label, matching Markdown's first-definition-wins behavior.
func (b *builder) collectMarkdownReferenceDefinitions(n *sitter.Node) {
	if n == nil {
		return
	}
	if n.Kind() == "link_reference_definition" {
		label := markdownLabel(b.directMarkdownChildText(n, "link_label"))
		if destination := b.markdownDestination(n); label != "" && destination != "" {
			if b.markdownReferences == nil {
				b.markdownReferences = make(map[string]string)
			}
			if _, exists := b.markdownReferences[label]; !exists {
				b.markdownReferences[label] = destination
			}
		}
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		b.collectMarkdownReferenceDefinitions(n.NamedChild(i))
	}
}

func (b *builder) markdownReferenceLabel(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	if label := b.directMarkdownChildText(n, "link_label"); label != "" {
		return markdownLabel(label)
	}
	switch n.Kind() {
	case "collapsed_reference_link", "shortcut_link":
		return markdownLabel(b.directMarkdownChildText(n, "link_text"))
	case "image":
		return markdownLabel(b.directMarkdownChildText(n, "image_description"))
	default:
		return ""
	}
}

func (b *builder) directMarkdownChildText(n *sitter.Node, kind string) string {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child.Kind() == kind {
			return b.nodeText(child)
		}
	}
	return ""
}

func markdownLabel(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		text = text[1 : len(text)-1]
	}
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func markdownText(text string) string {
	text = strings.Trim(strings.TrimSpace(text), "\"'")
	if strings.HasPrefix(text, "<") && strings.HasSuffix(text, ">") {
		text = strings.TrimSuffix(strings.TrimPrefix(text, "<"), ">")
	}
	return text
}

func (b *builder) importTexts(n *sitter.Node) []string {
	var out []string
	var add func(*sitter.Node)
	add = func(child *sitter.Node) {
		if child == nil {
			return
		}
		switch child.Kind() {
		case "dotted_name", "relative_import", "string", "string_literal":
			if text := cleanName(b.nodeText(child)); text != "" {
				out = append(out, text)
			}
		case "aliased_import":
			if text := cleanName(b.fieldText(child, "name")); text != "" {
				out = append(out, text)
			} else if text := cleanName(b.nodeText(child)); text != "" {
				if p := strings.Index(text, " as "); p >= 0 {
					text = text[:p]
				}
				out = append(out, text)
			}
		}
		for i := uint(0); i < child.NamedChildCount(); i++ {
			add(child.NamedChild(i))
		}
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		add(n.NamedChild(i))
	}
	return unique(out)
}

func importSourceText(text string) string {
	return strings.Trim(strings.TrimSpace(text), "'\"")
}

func (b *builder) typeScriptImportSource(n *sitter.Node) string {
	if source := b.fieldText(n, "source"); source != "" {
		return source
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child.Kind() == "import_require_clause" {
			return b.fieldText(child, "source")
		}
	}
	return ""
}

func (b *builder) expressionName(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case "identifier", "property_identifier", "private_property_identifier", "type_identifier", "dotted_name":
		return cleanName(b.nodeText(n))
	case "attribute", "member_expression", "field_expression":
		var left, right string
		for _, field := range []string{"object", "value", "attribute", "property", "field"} {
			if child := n.ChildByFieldName(field); child != nil {
				name := b.expressionName(child)
				if field == "attribute" || field == "property" || field == "field" {
					right = name
				} else {
					left = name
				}
			}
		}
		if left != "" && right != "" {
			return left + "." + right
		}
		if right != "" {
			return right
		}
		return left
	case "function_expression", "arrow_function", "parenthesized_expression", "non_null_expression":
		if n.NamedChildCount() == 1 {
			return b.expressionName(n.NamedChild(0))
		}
	}
	return ""
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := values[:0]
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
