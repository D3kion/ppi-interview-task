FROM golang:1.18-alpine

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN go build -v -o server

CMD ["./server"]
