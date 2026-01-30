# Build stage (Debian trixie, glibc 2.40)
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /geo ./cmd/geo

# Runtime stage — trixie-slim matches build glibc
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /geo /usr/local/bin/geo
COPY web/ /app/web/
WORKDIR /app
ENV SERVICE_PORT=8080
ENV SERVICE_WEB_DIR=/app/web
ENV SERVICE_DATA_DIR=/data
EXPOSE 8080
CMD ["geo"]
