# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git build-base
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/app ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/app /app/app
# Create data dirs for persistent storage
RUN mkdir -p /data/avatars /data/uploads /data/tls
ENV DATA_DIR=/data
ENV ENABLE_TLS=false
ENV HTTP_PORT=8081
ENV HTTPS_PORT=8443
CMD ["/app/app"]