FROM golang:1.17-alpine

WORKDIR /app

COPY . .

RUN go build -o membership main.go peerManager.go stateManager.go tcpCommunication.go udpCommunication.go util.go

ENTRYPOINT ["/app/membership"]