FROM golang:1.23-bookworm

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      build-essential \
      libusb-1.0.0-dev

WORKDIR /src
COPY . .
RUN go mod init go2dpfdemo
RUN go get
RUN go build -o ./demo main.go

FROM debian:bookworm
COPY --from=0 /src/demo /usr/bin/demo
CMD ["demo"]
