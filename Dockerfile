# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.26.4 AS build
WORKDIR /src

# Download modules first so the layer caches across source-only changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static, stripped binary; no cgo so it runs on the distroless static base.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/server

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
