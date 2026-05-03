#!/bin/bash
# Generate Go protobuf + gRPC code from .proto definitions.
# Run this from the project root.
set -euo pipefail

# Ensure protoc-gen-go and protoc-gen-go-grpc are on PATH
export PATH="${PATH}:$(go env GOPATH 2>/dev/null)/bin"

PROTO_DIR="api/proto"
OUT_DIR="proto"

protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" \
  --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/japanapi/v1/japanapi_v1.proto"

echo "Generated: ${OUT_DIR}/japanapi/v1/"
