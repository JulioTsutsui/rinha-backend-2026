FROM golang:1.26-alpine AS build

WORKDIR /build

COPY src/go.mod ./
RUN go mod download 2>/dev/null || true

COPY src/ ./
RUN GOAMD64=v3 CGO_ENABLED=0 go build \
      -ldflags="-s -w" \
      -trimpath \
      -pgo=auto \
      -o /out/server .

FROM alpine:3.20

WORKDIR /app/bin
COPY --from=build /out/server /app/bin/server
COPY resources /app/resources

EXPOSE 9999
ENTRYPOINT ["/app/bin/server"]
