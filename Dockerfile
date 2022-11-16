FROM golang:1.19.2-alpine as builder

WORKDIR /src/
RUN apk add --no-cache make git
ADD . .
RUN make build

FROM alpine:3.16.2

RUN apk add --no-cache util-linux e2fsprogs xfsprogs blkid e2fsprogs-extra xfsprogs-extra
COPY --from=builder /src/bin/nifcloud-hatoba-disk-csi-driver /bin/nifcloud-hatoba-disk-csi-driver
ENTRYPOINT ["/bin/nifcloud-hatoba-disk-csi-driver"]
