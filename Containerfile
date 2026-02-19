FROM docker.io/golang:1.26 as builder

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN curl https://mise.run | sh && /root/.local/bin/mise trust && /root/.local/bin/mise install

RUN /root/.local/bin/mise build


FROM registry.opensuse.org/opensuse/bci/bci-init:latest

LABEL org.opencontainers.image.description="Materia: a GitOps tool for managing Quadlets"
LABEL org.opencontainers.image.licenses=GPLv3


ARG TARGETARCH
WORKDIR /app
RUN mkdir -p /lib64
RUN mkdir -p /root/.ssh && \
	chmod 0700 /root/.ssh && \
	touch /root/.ssh/known_hosts

RUN zypper in -y podman && zypper clean

COPY --from=builder /go/src/app/materia/bin/materia-${TARGETARCH} /app/

ENTRYPOINT ["/app/materia"]
