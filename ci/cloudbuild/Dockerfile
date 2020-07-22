FROM golang:1.13.14-buster

RUN curl -L https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl > /bin/kubectl
RUN chmod 0750 /bin/kubectl

RUN mkdir /src
WORKDIR /src
RUN git clone https://github.com/kubernetes-sigs/kind /src/kind

WORKDIR /src/kind
RUN CGO_ENABLED=0 GO111MODULE=on go build -o /bin/kind

FROM gcr.io/cloud-builders/docker
COPY --from=0 /bin/kubectl /bin/kubectl
COPY --from=0 /bin/kind /bin/kind
# Remove default entrypoint
ENTRYPOINT []