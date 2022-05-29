FROM golang:1.17-alpine

WORKDIR /go/src/app
COPY . .

RUN apk add --no-cache ffmpeg libva-intel-driver

WORKDIR src
RUN go build -v .

CMD ["./recorder"]
