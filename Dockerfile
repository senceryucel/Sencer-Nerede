FROM golang:latest as builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sencer_nerede .

FROM alpine:latest  

# To be able to use timezones in Go - alpine doesn't have them by default
RUN apk add --no-cache tzdata 

WORKDIR /root/
COPY --from=builder /app/sencer_nerede .

CMD ["./sencer_nerede"]
