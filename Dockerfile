FROM debian:12.5

LABEL version="v1.0" app="kiosk"

RUN mkdir /app && apt update && apt install ca-certificates -y
COPY ./src/bin/kiosk /app/kiosk

RUN ln -s /app/kiosk /bin/kiosk 


WORKDIR /app

ENTRYPOINT ["kiosk"]
