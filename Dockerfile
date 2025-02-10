FROM golang:latest
LABEL maintainer="mintyleaf <mintyleafdev@gmail.com>"

WORKDIR /app

COPY go.mod go.sum ./
COPY . .
RUN go mod download
RUN go build -o bot cmd/bot/main.go

CMD ["./bot"]
