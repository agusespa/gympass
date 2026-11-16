# syntax=docker/dockerfile:1

# Frontend: platform-independent output, so build it once on the build host.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json web/.npmrc ./
RUN npm ci
COPY web/ ./
# The built-in exercises, shared with the server.
COPY internal/store/catalog.json /src/internal/store/catalog.json
# The release shown in Settings; .git isn't in the build context, so CI passes the tag.
ARG APP_VERSION=dev
RUN APP_VERSION=$APP_VERSION npm run build

# Go binary: pure Go (no cgo), cross-compiled for the target platform.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS server
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/web/build ./internal/ui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/gym-tracker ./cmd/gym-tracker

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=server /out/gym-tracker /app/gym-tracker
ENV DB_PATH=/app/data/gym.db \
    PORT=8080
EXPOSE 8080
VOLUME /app/data
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/gym-tracker", "healthcheck"]
ENTRYPOINT ["/app/gym-tracker"]
