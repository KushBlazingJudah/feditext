FROM golang:1.18-alpine AS builder

RUN apk add sqlite-dev gcc musl-dev make git
COPY . /src
WORKDIR /src
RUN make build

FROM alpine:3.16
RUN apk --no-cache add sqlite ca-certificates && mkdir /app
COPY --from=builder /src/feditext /app
COPY ./views/ /app/views/
COPY ./static/ /app/static/
CMD ["/bin/sh", "-c", "cd /app && exec ./feditext"]
