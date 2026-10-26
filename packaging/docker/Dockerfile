# syntax=docker/dockerfile:1
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY haepage /usr/local/bin/haepage
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/haepage"]
