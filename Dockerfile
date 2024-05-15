FROM golang:1.22.3 as builder
ARG GOARCH

WORKDIR /metal-provider

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY internal/ internal/
COPY servers/ servers/
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -a -o metal-provider .

FROM debian:bookworm-20240513-slim

WORKDIR /

USER 65532:65532
ENTRYPOINT ["/metal-provider"]

COPY --from=builder /metal-provider/metal-provider .
