FROM golang:1.22 as builder

WORKDIR /app
COPY . .

ENV CGO_ENABLED 0
ENV GOOS linux

RUN go mod download
RUN go build -o kiosk ./cmd/kiosk/main.go

#------------------------------------------

FROM debian:12.5

LABEL version="v1.0" app="kiosk"

WORKDIR /app
COPY --from=builder /app/kiosk .

RUN apt update && apt install ca-certificates -y

# Для логов корректируем часовой пояс
RUN apt install tzdata
RUN rm /etc/localtime
RUN ln -s /usr/share/zoneinfo/Europe/Moscow /etc/localtime

ENTRYPOINT ["/app/kiosk"]