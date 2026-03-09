# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.23 AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/intern-api ./cmd/intern-api

FROM alpine:3.23

WORKDIR /app

COPY --from=build /out/intern-api /usr/local/bin/intern-api

EXPOSE 8080

ENTRYPOINT ["intern-api"]
