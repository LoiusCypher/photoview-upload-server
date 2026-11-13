ARG ARCH=amd64
FROM golang:latest AS build

LABEL org.opencontainers.image.authors="Mei Akizuru <chimeaquas@hotmail.com>"

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

# resolve dependency before copying whole source code
RUN apt update
RUN apt install --no-install-recommends -y ghostscript sqlite3
COPY go.mod go.sum ./
RUN go clean -modcache
#RUN go get ./photoviewuploadserver
RUN go get github.com/stretchr/testify/assert && cat go.mod go.sum
#RUN go get github.com/stretchr/testify && cat go.mod go.sum
#RUN go get github.com/LoiusCypher/photoview-upload-server/uploadserver
RUN go mod download
RUN go mod tidy
RUN cat go.mod go.sum

# copy other sources & build
COPY . /go/src/app
RUN GOOS=linux GOARCH=${ARCH} CGO_ENABLED=1 go build -o /go/bin/app
RUN cat go.mod go.sum

# # hadolint ignore=DL3007 # distroless does not provide fixed tag.
# FROM gcr.io/distroless/static-debian13:latest
# COPY --from=build /go/bin/app /usr/local/bin/app
# ENTRYPOINT ["/usr/local/bin/app"]

# hadolint ignore=DL3007 # distroless does not provide fixed tag.
FROM debian:trixie-slim
RUN apt update
RUN apt install --no-install-recommends -y ghostscript sqlite3
COPY --from=build /go/bin/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]

