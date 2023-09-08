package parser

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type TypeDefinition struct {
	Kind  string
	Group string

	Scope string

	Schema *OpenAPI
}

func buildOpenAPISchema(ctx context.Context, n Node) ([]TypeDefinition, error) {
	b := &openAPIBuilder{}

	schemas := b.buildOpenAPISchema(n)

	if b.errors != nil {
		return nil, errors.Join(b.errors...)
	}
	return schemas, nil
}

type openAPIBuilder struct {
	builder
}

func (b *openAPIBuilder) buildOpenAPISchema(node Node) []TypeDefinition {
	if b.Err() != nil {
		return nil
	}

	var schemas []TypeDefinition

	switch node := node.(type) {
	case *ObjectNode:
		for _, entry := range node.Entries {
			name, args := b.parseArgs(b.expectString(entry.Key))
			v := b.expectObject(entry.Value)

			schema := b.buildOpenAPIObject(v)
			info := TypeDefinition{
				Kind:   name,
				Schema: &OpenAPI{Schema: schema},
			}

			for k, v := range args {
				switch k {
				case "group":
					info.Group = b.argToString(v)
				case "scope":
					info.Scope = b.argToString(v)
				default:
					b.Errorf("unknown parameter %q", k)
				}
			}

			schemas = append(schemas, info)
		}

	default:
		b.Errorf("expected object, got %T", node)
	}
	return schemas
}

func (b *openAPIBuilder) expectObject(node Node) *ObjectNode {
	if b.Err() != nil {
		return nil
	}

	switch node := node.(type) {
	case *ObjectNode:
		return node
	default:
		b.Errorf("unexpected type, want object, got %T", node)
	}
	return nil
}

func (b *openAPIBuilder) expectString(node Node) string {
	if b.Err() != nil {
		return ""
	}

	switch node := node.(type) {
	case *ScalarNode:
		return node.Value
	default:
		b.Errorf("unexpected type, want string, got %T", node)
		return ""
	}
}

func (b *openAPIBuilder) argToString(arg any) string {
	if b.Err() != nil {
		return ""
	}

	switch arg := arg.(type) {
	case string:
		// TODO: Move to parser (and de-escape there?)
		if len(arg) != 0 && arg[0] == '"' && arg[len(arg)-1] == '"' {
			arg = arg[1 : len(arg)-1]
		}
		return arg
	default:
		b.Errorf("unexpected type, want string, got %T", arg)
		return ""
	}
}

func (b *openAPIBuilder) buildOpenAPIProperty(v Node) *Property {
	prop := &Property{}

	if b.Err() != nil {
		return prop
	}

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
			b.Errorf("unhandled string value %q", s)
		}

		for k, v := range args {
			switch k {
			case "min":
				sv := b.argToString(v)
				n, err := strconv.Atoi(sv)
				if err != nil {
					b.Errorf("invalid min value (not an integer) %q", sv)
				}
				prop.Minimum = &n
			case "key":
				// TODO: Not represented in OpenAPI?
			case "value":
				// TODO: prop.AdditionalProperties.Type = "string"

			case "regex":
				sv := b.argToString(v)
				prop.Pattern = &sv

			default:
				b.Errorf("unknown parameter %q", k)
			}
		}
	case *ObjectNode:
		p := b.buildOpenAPIObject(v)
		prop = p

	case *SequenceNode:
		p := b.buildOpenAPIArray(v)
		prop = p

	default:
		b.Errorf("unhandled type %T", v)
	}

	return prop
}

func (b *openAPIBuilder) buildOpenAPIObject(def *ObjectNode) *Property {
	if b.Err() != nil {
		return &Property{}
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
				prop.XKubernetesListMapKeys = []string{b.argToString(v)}
			default:
				b.Errorf("unknown parameter %q", k)
			}
		}
	}

	return schema
}

func (b *openAPIBuilder) buildOpenAPIArray(def *SequenceNode) *Property {
	if b.Err() != nil {
		return &Property{}
	}

	if len(def.Entries) != 1 {
		b.Errorf("expected exactly one entry in array, got %v", def)
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
		b.Errorf("expected args to start with '('")
		return name, nil
	}
	s = s[1:]
	if !strings.HasSuffix(s, ")") {
		b.Errorf("expected args to end with ')'")
		return name, nil
	}
	s = s[:len(s)-1]

	args := make(map[string]any)

	tokens := strings.Split(s, ",")
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		ix := strings.Index(token, "=")
		if ix == -1 {
			b.Errorf("cannot parse parameter %q (expected key = value)", token)
			return name, nil
		}
		k := strings.TrimSpace(token[:ix])
		v := strings.TrimSpace(token[ix+1:])
		args[k] = v
	}
	return name, args
}
