generate-typeinfo is a simple tool to generate the kubernetes_builtin_schema.meta.yaml from a kubernetes OpenAPI definition.

The openapi definition should be saved as `openapi.json`, using a command like `kubectl get --raw /openapi/v2 > openapi.json`.

The output path is currently hard-coded to `../../kubernetes_builtin_schema.meta.yaml`, so should be run from this directory.