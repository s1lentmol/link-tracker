FROM golang:1.25.2 AS builder

WORKDIR /app
ENV GOPROXY=direct
ENV GOSUMDB=off

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/scrapper ./cmd/scrapper

FROM alpine:3.21.3

WORKDIR /app

COPY --from=builder /bin/scrapper /app/scrapper
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

CMD ["./scrapper"]
