FROM alpine:3.18.2

RUN apk update && apk upgrade && apk add --no-cache bash nvme-cli bash util-linux
ADD manage-startup-script.sh /
CMD /manage-startup-script.sh