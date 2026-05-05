FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/backfill_units ./cmd/backfill_units

FROM alpine:3.20
RUN adduser -D -H appuser
USER appuser
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/backfill_units /usr/local/bin/backfill_units
ENTRYPOINT ["/usr/local/bin/api"]
