package parser

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func buildOpenAPISchema(ctx context.Context, n Node) (map[string]*OpenAPI, error) {
	b := &openAPIBuilder{}

	schemas := b.buildOpenAPISchema(n)

	if b.errors != nil {
		return nil, errors.Join(b.errors...)
	}
	return schemas, nil
}

type openAPIBuilder struct {
	errors []error
}

func (p *openAPIBuilder) errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	p.errors = append(p.errors, err)
}

func (b *openAPIBuilder) buildOpenAPISchema(node Node) map[string]*OpenAPI {
	if len(b.errors) != 0 {
		return nil
	}

	schemas := make(map[string]*OpenAPI)

	switch node := node.(type) {
	case *ObjectNode:
		for _, entry := range node.Entries {
			name := b.expectString(entry.Key)
			v := b.expectObject(entry.Value)

			schema := b.buildOpenAPIObject(v)
			schemas[name] = &OpenAPI{Schema: schema}
		}

	default:
		b.errorf("expected object, got %T", node)
	}
	return schemas
}

func (b *openAPIBuilder) expectObject(node Node) *ObjectNode {
	if len(b.errors) != 0 {
		return nil
	}

	switch node := node.(type) {
	case *ObjectNode:
		return node
	default:
		b.errorf("unexpected type, want object, got %T", node)
	}
	return nil
}

func (b *openAPIBuilder) expectString(node Node) string {
	if len(b.errors) != 0 {
		return ""
	}

	switch node := node.(type) {
	case *ScalarNode:
		return node.Value
	default:
		b.errorf("unexpected type, want string, got %T", node)
		return ""
	}
}

func (b *openAPIBuilder) buildOpenAPIProperty(v Node) *Property {
	if len(b.errors) != 0 {
		return nil
	}

	var prop *Property
	switch v := v.(type) {
	case *ScalarNode:
		s := v.Value
		name, args := b.parseArgs(s)

		switch name {
		case "map":
			prop = &Property{Type: "object"}
		case "int":
			prop = &Property{Type: "integer"}
		case "string":
			prop = &Property{Type: "string"}
		default:
			b.errorf("unhandled string value %q", s)
		}

		for k, v := range args {
			switch k {
			case "min":
				sv := v.(string)
				n, err := strconv.Atoi(sv)
				if err != nil {
					b.errorf("invalid min value (not an integer) %q", sv)
				}
				prop.Minimum = &n
			case "key":
				// TODO: Not represented in OpenAPI?
			case "value":
				// TODO: prop.AdditionalProperties.Type = "string"

			case "regex":
				sv := v.(string)
				prop.Pattern = &sv

			default:
				b.errorf("unknown parameter %q", k)
			}
		}
	case *ObjectNode:
		p := b.buildOpenAPIObject(v)
		prop = p

	case *SequenceNode:
		p := b.buildOpenAPIArray(v)
		prop = p

	default:
		b.errorf("unhandled type %T", v)
	}

	return prop
}

func (b *openAPIBuilder) buildOpenAPIObject(def *ObjectNode) *Property {
	if len(b.errors) != 0 {
		return nil
	}

	schema := &Property{
		Type:       "object",
		Properties: make(map[string]*Property),
	}

	for _, entry := range def.Entries {
		k := b.expectString(entry.Key)
		name, args := b.parseArgs(k)

		prop := b.buildOpenAPIProperty(entry.Value)
		if prop == nil {
			continue
		}
		prop.Description = joinComments(entry.Key.GetComment(), entry.Value.GetComment())
		schema.Properties[name] = prop

		for k, v := range args {
			switch k {
			case "map-key":
				prop.XKubernetesListType = "map"
				prop.XKubernetesListMapKeys = []string{v.(string)}
			default:
				b.errorf("unknown parameter %q", k)
			}
		}
	}

	return schema
}

func (b *openAPIBuilder) buildOpenAPIArray(def *SequenceNode) *Property {
	if len(b.errors) != 0 {
		return nil
	}

	if len(def.Entries) != 1 {
		b.errorf("expected exactly one entry in array, got %v", def)
		return nil
	}

	schema := &Property{
		Type: "array",
	}

	item := b.buildOpenAPIProperty(def.Entries[0])
	schema.Items = item

	return schema
}

type OpenAPI struct {
	Schema *Property `json:"openAPIV3Schema,omitempty"`
}

// type OpenAPIV3Schema struct {
// 	Description string               `json:"description,omitempty"`
// 	Properties  map[string]*Property `json:"properties,omitempty"`
// }

type Property struct {
	Description string               `json:"description,omitempty"`
	Type        string               `json:"type,omitempty"`
	Items       *Property            `json:"items,omitempty"`
	Properties  map[string]*Property `json:"properties,omitempty"`
	Required    []string             `json:"required,omitempty"`
	Minimum     *int                 `json:"minimum,omitempty"`
	Pattern     *string              `json:"pattern,omitempty"`

	XKubernetesListType    string   `json:"x-kubernetes-list-type,omitempty"`
	XKubernetesListMapKeys []string `json:"x-kubernetes-list-map-keys,omitempty"`
}

func PtrTo[T any](t T) *T {
	return &t
}

func (b *openAPIBuilder) parseArgs(s string) (string, map[string]any) {
	ix := strings.Index(s, "(")
	if ix == -1 {
		return s, nil
	}

	name := s[:ix]
	s = s[ix:]
	if !strings.HasPrefix(s, "(") {
		b.errorf("expected args to start with '('")
		return name, nil
	}
	s = s[1:]
	if !strings.HasSuffix(s, ")") {
		b.errorf("expected args to end with ')'")
		return name, nil
	}
	s = s[:len(s)-1]

	args := make(map[string]any)

	tokens := strings.Split(s, ",")
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		ix := strings.Index(token, "=")
		if ix == -1 {
			b.errorf("cannot parse parameter %q (expected key = value)", token)
			return name, nil
		}
		k := strings.TrimSpace(token[:ix])
		v := strings.TrimSpace(token[ix+1:])
		args[k] = v
	}
	return name, args
}
