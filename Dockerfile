FROM golang:1.22.3 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /metal-provider
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o metal-provider cmd/main.go

FROM debian:bookworm-20240513-slim
WORKDIR /
USER 65532:65532
ENTRYPOINT ["/metal-provider"]

COPY --from=builder /metal-provider/metal-provider .
