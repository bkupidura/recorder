FROM golang:1.22-alpine

WORKDIR /go/src/app
COPY . .

RUN apk add --no-cache ffmpeg libva-intel-driver

RUN go build -v .

CMD ["./recorder"]
