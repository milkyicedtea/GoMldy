FROM golang:latest

ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

WORKDIR /app

RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/list/*

COPY . .

RUN go mod download

RUN go build -o /app/GoMldy

EXPOSE 9000

CMD ["/app/GoMldy"]