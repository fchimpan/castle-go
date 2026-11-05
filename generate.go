package castle

//go:generate go run ./internal/cmd/prepareapi
//go:generate go run -modfile=internal/tools/go.mod github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config api/oapi-codegen.yaml api/prepared.tmp.json
//go:generate go run ./internal/cmd/generateclient
//go:generate go run ./internal/cmd/prepareapi -clean
