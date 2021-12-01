# Build the manager binary
FROM golang:1.17 as builder

# Copy in the go src
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
COPY vendor/   vendor/

COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY channels/ channels/
RUN chmod -R a+rx channels/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Copy the operator and dependencies into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/channels/ channels/

USER 65532:65532

ENTRYPOINT ["./manager"]
