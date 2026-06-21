FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

RUN corepack enable
ENV CYPRESS_INSTALL_BINARY=0

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend ./

ARG VITE_APP_VERSION=latest
ENV VITE_APP_VERSION=$VITE_APP_VERSION

RUN pnpm build:main && pnpm build:widget


FROM golang:1.25-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates
RUN go install github.com/knadh/stuffbin/...@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

ARG LIBREDESK_VERSION=v0.0.0

RUN CGO_ENABLED=0 go build \
  -ldflags="-X 'main.buildString=${LIBREDESK_VERSION}' -X 'main.versionString=${LIBREDESK_VERSION}' -X 'github.com/abhinavxd/libredesk/internal/version.Version=${LIBREDESK_VERSION}' -s -w" \
  -o libredesk ./cmd

RUN /root/go/bin/stuffbin -a stuff -in libredesk -out libredesk frontend/dist i18n schema.sql static


FROM alpine:3.18

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /libredesk

COPY --from=backend-builder /app/libredesk ./libredesk
COPY config.sample.toml ./config.toml

EXPOSE 9000

CMD ["./libredesk", "--config", "/libredesk/config.toml"]