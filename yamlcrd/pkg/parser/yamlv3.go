package parser

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func Parse(ctx context.Context, b []byte) (Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("error parsing: %w", err)
	}

	p := &parser{data: b}

	node := p.buildNode(&doc)

	if p.errors != nil {
		return nil, errors.Join(p.errors...)
	}
	return node, nil
}

type parser struct {
	data   []byte
	errors []error
}

func (p *parser) errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	p.errors = append(p.errors, err)
}

func (p *parser) expectChildren(n *yaml.Node, children int) {
	if len(n.Content) != children {
		p.errorf("expected exactly %d children for %v, got %d", children, n, len(n.Content))
	}
}

func (p *parser) buildNode(in *yaml.Node) Node {
	if len(p.errors) != 0 {
		return nil
	}

	n := len(in.Content)
	switch in.Kind {
	case yaml.MappingNode:
		node := &ObjectNode{}
		if n%2 != 0 {
			p.errorf("expected even number of children, got %d", n)
			return nil
		}
		for i := 0; i < n; i += 2 {
			key := p.buildNode(in.Content[i])
			value := p.buildNode(in.Content[i+1])
			node.Entries = append(node.Entries, &ObjectNodeKeyValue{Key: key, Value: value})
		}
		return node

	case yaml.SequenceNode:
		node := &SequenceNode{}
		for i := 0; i < n; i++ {
			value := p.buildNode(in.Content[i])
			node.Entries = append(node.Entries, value)
		}
		return node

	case yaml.ScalarNode:
		p.expectChildren(in, 0)

		node := &ScalarNode{}
		node.Value = in.Value
		node.Comment = joinYAMLComments(in.HeadComment, in.FootComment, in.LineComment)
		return node

	case yaml.DocumentNode:
		p.expectChildren(in, 1)
		return p.buildNode(in.Content[0])

	default:
		p.errorf("unhandled node kind %v", in.Kind)

		return nil
	}
}

func joinComments(comments ...string) string {
	if len(comments) == 0 {
		return ""
	}
	joined := comments[0]
	for i := 1; i < len(comments); i++ {
		s := comments[i]
		if joined == "" {
			joined = s
		} else {
			joined = joined + "\n" + s
		}
	}
	return strings.TrimSpace(joined)
}

func joinYAMLComments(comments ...string) string {
	for i := range comments {
		if strings.HasPrefix(comments[i], "#") {
			comments[i] = strings.TrimSpace(strings.TrimPrefix(comments[i], "#"))
		}
	}
	return joinComments(comments...)
}
