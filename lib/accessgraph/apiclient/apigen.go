package accessgraph

//go:generate oapi-codegen -config ./models/oapi-codegen.cfg.yaml -o ./models/graph/models.gen.go ./openapi/models/graph.yaml
//go:generate oapi-codegen -config ./models/oapi-codegen.cfg.yaml -o ./models/jsondiff/models.gen.go ./openapi/models/json-diff.yaml
//go:generate oapi-codegen -config ./models/oapi-codegen.cfg.yaml -o ./models/logs/models.gen.go ./openapi/models/logs.yaml
//go:generate oapi-codegen -config oapi-codegen.cfg.yaml -o client.gen.go openapi.yaml
