FROM golang:1.24-alpine as builder
WORKDIR /go/src/app
COPY . .
RUN apk add --no-cache ca-certificates make
RUN CGO_ENABLED=0 make tools && make

FROM alpine:latest as final
WORKDIR /app
RUN mkdir -p /lib64
COPY --from=builder /go/src/app/materia /app/

CMD ["/app/materia"]
