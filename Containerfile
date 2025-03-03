FROM registry.opensuse.org/opensuse/bci/golang:1.24 as builder
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 make tools && make

FROM registry.opensuse.org/opensuse/tumbleweed:latest as final
WORKDIR /app
RUN mkdir -p /lib64
RUN touch /root/.ssh/known_hosts
RUN zypper in -y podman git

COPY --from=builder /go/src/app/materia /app/

ENTRYPOINT ["/app/materia"]
