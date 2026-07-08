package skills

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the SKILL.md fields huly reads or writes.
type Frontmatter struct {
	Name        string
	Description string
	ManagedBy   string // metadata.managed_by
	Version     string // metadata.huly_cli_version
	ContentHash string // metadata.content_hash
}

var fence = []byte("---")

// Split separates a SKILL.md into its YAML frontmatter block and the body.
// front is the bytes between the first two "---" fence lines (no fences);
// body is everything after the line following the second fence. ok is false
// when src does not begin with a "---" fence.
func Split(src []byte) (front, body []byte, ok bool) {
	lines := bytes.SplitAfter(src, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimRight(lines[0], "\r\n"), fence) {
		return nil, nil, false
	}
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimRight(lines[i], "\r\n"), fence) {
			front = bytes.Join(lines[1:i], nil)
			body = bytes.Join(lines[i+1:], nil)
			return front, body, true
		}
	}
	return nil, nil, false
}

// fmYAML mirrors the subset of frontmatter we parse.
type fmYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		ManagedBy   string `yaml:"managed_by"`
		Version     string `yaml:"huly_cli_version"`
		ContentHash string `yaml:"content_hash"`
	} `yaml:"metadata"`
}

// Parse reads the fields huly cares about from a SKILL.md's raw bytes.
func Parse(src []byte) (Frontmatter, error) {
	front, _, ok := Split(src)
	if !ok {
		return Frontmatter{}, fmt.Errorf("no frontmatter fence")
	}
	var y fmYAML
	if err := yaml.Unmarshal(front, &y); err != nil {
		return Frontmatter{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	return Frontmatter{
		Name:        y.Name,
		Description: y.Description,
		ManagedBy:   y.Metadata.ManagedBy,
		Version:     y.Metadata.Version,
		ContentHash: y.Metadata.ContentHash,
	}, nil
}

// Stamp sets metadata.managed_by/huly_cli_version/content_hash on a SKILL.md,
// emitting the values quoted, preserving the body byte-for-byte and the rest
// of the frontmatter's key order. It rebuilds only the frontmatter via a
// yaml.Node so authored keys keep their order.
//
// NOTE: only the BODY is byte-preserved. The frontmatter is re-emitted by the
// yaml encoder, so a hand-wrapped folded ('>-') description collapses onto one
// line on the first install. This is intentional and harmless: the content
// hash excludes the frontmatter entirely (see hash.go), so the reflow is
// cosmetic and never triggers a false "modified".
func Stamp(src []byte, managedBy, version, contentHash string) ([]byte, error) {
	front, body, ok := Split(src)
	if !ok {
		return nil, fmt.Errorf("no frontmatter fence")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(front, &doc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter is not a mapping")
	}
	root := doc.Content[0]
	meta := mappingValue(root, "metadata")
	if meta.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter metadata is not a mapping")
	}
	setScalar(meta, "managed_by", managedBy)
	setScalar(meta, "huly_cli_version", version)
	setScalar(meta, "content_hash", contentHash)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	_ = enc.Close()

	out := make([]byte, 0, len(buf.Bytes())+len(body)+8)
	out = append(out, "---\n"...)
	out = append(out, buf.Bytes()...)
	out = append(out, "---\n"...)
	out = append(out, body...)
	return out, nil
}

// mappingValue returns the value node for key in a mapping node, creating an
// empty mapping value if the key is absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, k, v)
	return v
}

// setScalar sets key=val (double-quoted string) in a mapping node.
func setScalar(m *yaml.Node, key, val string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			v.Kind, v.Tag, v.Value, v.Style = yaml.ScalarNode, "!!str", val, yaml.DoubleQuotedStyle
			return
		}
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	v := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val, Style: yaml.DoubleQuotedStyle}
	m.Content = append(m.Content, k, v)
}
