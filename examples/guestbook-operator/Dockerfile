FROM ubuntu:latest as kubectl
RUN apt-get update
RUN apt-get install -y curl
RUN curl -fsSL https://dl.k8s.io/release/v1.13.4/bin/linux/amd64/kubectl > /usr/bin/kubectl
RUN chmod a+rx /usr/bin/kubectl

# Build the manager binary
FROM golang:1.13 as builder

# Copy in the go src
WORKDIR /go/src/guestbook-operator
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

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Copy the operator and dependencies into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /go/src/guestbook-operator/manager .
COPY --from=kubectl /usr/bin/kubectl /usr/bin/kubectl
COPY channels/ channels/
ENTRYPOINT ["./manager"]
