LABEL org.opencontainers.image.description Materia: a GitOps tool for managing Quadlets

FROM registry.opensuse.org/opensuse/bci/golang:1.24 as builder
WORKDIR /go/src/app
COPY . .
RUN curl https://mise.run | sh && /root/.local/bin/mise trust && /root/.local/bin/mise install && go install golang.org/x/tools/cmd/stringer
RUN /root/.local/bin/mise build

FROM registry.opensuse.org/opensuse/tumbleweed:latest as final
WORKDIR /app
RUN mkdir -p /lib64
RUN mkdir -p /root/.ssh && \
	chmod 0700 /root/.ssh && \
	touch /root/.ssh/known_hosts

RUN zypper in -y podman git openssh openssh-clients

COPY --from=builder /go/src/app/materia /app/

ENTRYPOINT ["/app/materia"]
