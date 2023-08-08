FROM golang:alpine AS build
RUN apk --no-cache add gcc g++ make git
WORKDIR /go/src/app
COPY . .
RUN go mod init webserver
RUN go mod tidy
RUN GOOS=linux go build -ldflags="-s -w" -o ./bin/gosocks ./main.go

FROM alpine:3.17
RUN apk --no-cache add ca-certificates
WORKDIR /usr/bin
COPY --from=build /go/src/app/bin /go/bin
EXPOSE 80
EXPOSE 443
ENTRYPOINT /go/bin/gosocks --port 80