package parser

type Node interface {
	GetComment() string
}

type BaseNode struct {
	Comment string
}
type ObjectNode struct {
	BaseNode
	Entries []*ObjectNodeKeyValue
}

type SequenceNode struct {
	BaseNode
	Entries []Node
}

type ObjectNodeKeyValue struct {
	Key   Node
	Value Node
}

type ScalarNode struct {
	BaseNode
	Value string
}

func (b *BaseNode) GetComment() string {
	return b.Comment
}
