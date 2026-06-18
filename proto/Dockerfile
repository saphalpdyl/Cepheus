# Reproducible buf + Go codegen toolchain.
# Referenced from: https://buf.build/docs/cli/installation/#docker
#
# Plugin versions are pinned to the runtime versions in the root go.mod so the
# generated code stays compatible with what the services compile against:
#   - protoc-gen-go         <-> google.golang.org/protobuf
#   - protoc-gen-connect-go <-> connectrpc.com/connect
# Bump these together with the corresponding go.mod entries.

FROM bufbuild/buf:latest AS buf

FROM golang:1.25-alpine

# Copy buf binary from the official image
COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf

# Install pinned Go protoc-gen plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.20.0

RUN adduser -D bufuser
WORKDIR /workspace
USER bufuser
ENTRYPOINT ["buf"]
