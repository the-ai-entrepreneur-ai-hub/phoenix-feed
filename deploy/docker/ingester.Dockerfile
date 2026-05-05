FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ingester ./cmd/ingester

FROM alpine:3.20
RUN adduser -D -H appuser
USER appuser
COPY --from=build /out/ingester /usr/local/bin/ingester
ENTRYPOINT ["/usr/local/bin/ingester"]
