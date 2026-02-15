FROM golang:1.24-alpine

RUN apk add --no-cache ca-certificates curl

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8080

CMD ["go", "run", "./cmd/server"]
