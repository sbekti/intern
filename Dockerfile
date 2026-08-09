# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.23 AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY tools/go.mod tools/go.sum ./tools/

COPY cmd ./cmd
COPY db ./db
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/intern-api ./cmd/intern-api
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -modfile=tools/go.mod -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM alpine:3.23

WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/intern-api /usr/local/bin/intern-api
COPY --from=build /out/goose /usr/local/bin/goose
COPY --from=build /src/db/migrations ./db/migrations

EXPOSE 8080
ENTRYPOINT ["intern-api"]
