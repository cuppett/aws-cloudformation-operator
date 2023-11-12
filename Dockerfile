# Build the manager binary
FROM quay.io/cuppett/golang:1.21 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use ubi8/ubi-minimal as minimal base image to package the manager binary
FROM ghcr.io/cuppett/fedora-minimal:latest

LABEL maintainer "Stephen Cuppett <steve@cuppett.com>" \
      org.opencontainers.image.title "aws-cloudformation-operator" \
      org.opencontainers.image.source "https://github.com/cuppett/aws-cloudformation-operator"

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
