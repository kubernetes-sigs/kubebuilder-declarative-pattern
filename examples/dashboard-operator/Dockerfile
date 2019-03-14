FROM ubuntu:latest as kubectl
RUN apt-get update
RUN apt-get install -y curl
RUN curl -fsSL https://dl.k8s.io/release/v1.13.4/bin/linux/amd64/kubectl > /usr/bin/kubectl
RUN chmod a+rx /usr/bin/kubectl

# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/kubebuilder-declarative-pattern/examples/dashboard-operator
COPY pkg/      pkg/
COPY cmd/      cmd/
COPY vendor/   vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager sigs.k8s.io/kubebuilder-declarative-pattern/examples/dashboard-operator/cmd/manager

# Copy the controller-manager into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /root/
COPY --from=builder /go/src/sigs.k8s.io/kubebuilder-declarative-pattern/examples/dashboard-operator/manager .
COPY --from=kubectl /usr/bin/kubectl /usr/bin/kubectl
COPY channels/ channels/
ENTRYPOINT ["./manager"]
