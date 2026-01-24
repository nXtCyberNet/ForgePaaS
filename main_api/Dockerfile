
FROM golang:1.25 AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download

COPY api.go . /app/


RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o app


FROM gcr.io/distroless/base-debian12

WORKDIR /app


COPY --from=builder /app/app /app/app

USER nonroot:nonroot

ENV APP_ENV=production \
    APP_PORT=8080 \
    REDIS_URL=

EXPOSE 8080

ENTRYPOINT ["/app/app"]
