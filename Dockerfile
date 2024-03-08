FROM golang:1.22.1 as builder
ARG GOARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY hack/ hack/
RUN --mount=type=ssh --mount=type=secret,id=github_pat GITHUB_PAT_PATH=/run/secrets/github_pat ./hack/setup-git-redirect.sh \
  && mkdir -p -m 0600 ~/.ssh \
  && ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts \
  && go mod download

COPY apis/ apis/
COPY client/applyconfiguration/ client/applyconfiguration/
COPY metal-provider/ metal-provider/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -a -o /metal-provider metal-provider/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -a -o /irictl-machine github.com/ironcore-dev/ironcore/irictl-machine/cmd/irictl-machine

FROM debian:bookworm-20240211-slim
WORKDIR /
USER 65532:65532
ENTRYPOINT ["/metal-provider"]

COPY --from=builder /metal-provider .
COPY --from=builder /irictl-machine .
