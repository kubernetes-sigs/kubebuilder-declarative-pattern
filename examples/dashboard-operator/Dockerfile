# Fetch kubectl
FROM ubuntu:latest as kubectl
RUN apt-get update
RUN apt-get install -y curl
RUN curl -fsSL https://dl.k8s.io/release/v1.13.4/bin/linux/amd64/kubectl > /usr/bin/kubectl
RUN chmod a+rx /usr/bin/kubectl

# Build the manager binary
FROM golang:1.12.5 as builder

WORKDIR /workspace/src/dashboard-operator

# Use the vendor directory generated as part of the build
# This is necessary because we need to import from the same codebase we're running in
COPY vendor/ vendor/

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPATH=/workspace go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /workspace/src/dashboard-operator/manager .
COPY --from=kubectl /usr/bin/kubectl /usr/bin/kubectl
COPY channels/ channels/
ENTRYPOINT ["/manager"]
