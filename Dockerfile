# Build the manager binary
FROM quay.io/cuppett/golang:1.19 as builder

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
# https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/blob/master/docs/addon/walkthrough/README.md#adding-a-manifest
# Stage channels and make readable
COPY channels/ /channels/
RUN chmod -R a+rx /channels/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use ubi8/ubi-minimal as minimal base image to package the manager binary
FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /
COPY --from=builder /workspace/manager .
# copy channels
COPY --from=builder /channels /channels
USER 65532:65532

ENTRYPOINT ["/manager"]
