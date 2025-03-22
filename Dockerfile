FROM golang:1.24.1 AS builder
COPY . /app
RUN cd /app && go mod verify && go build

FROM debian:bookworm
COPY --from=builder /app/kvitto-tcp /usr/bin/kvitto-tcp

CMD ["/usr/bin/kvitto-tcp"]
