FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY app ./app
RUN CGO_ENABLED=0 GOOS=linux go build -o /redis-server ./app

FROM alpine:3.21

COPY --from=build /redis-server /redis-server

EXPOSE 6379

ENTRYPOINT ["/redis-server"]
