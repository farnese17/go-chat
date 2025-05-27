FROM golang:1.23.3-alpine AS builder
ENV APPDIR=/gochat
ENV APPName=gochat
WORKDIR ${APPDIR}
COPY . .
RUN go mod download
RUN export VERSION=`cat VERSION` && \
    GOOS=linux GOARCH=amd64 go build -o bin/${APPName} -ldflags "-X github.com/farnese17/chat/cli.Version=$VERSION" ./main.go 

FROM alpine:latest
ENV APPDIR=/gochat
ENV APPName=gochat
WORKDIR ${APPDIR}
COPY --from=builder ${APPDIR}/bin/${APPName} ${APPDIR}
RUN mkdir -p log config storage/log storage/files
RUN addgroup -S gochat && adduser -S gochat -G gochat
RUN chmod +x ${APPName} && \
    chown -R gochat:gochat ${APPDIR}
USER gochat
CMD [ "./gochat" ]