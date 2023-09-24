package memorystorage

import (
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/schemas"
)

type mockSchema struct {
	builtin   *schemas.Schema
	resources []*memoryResourceInfo
}

func (s *mockSchema) Init() error {
	schema, err := schemas.KubernetesBuiltInSchema()
	if err != nil {
		return err
	}
	s.builtin = schema
	return nil
}
