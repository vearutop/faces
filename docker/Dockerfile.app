FROM goface:latest as builder
WORKDIR /go/src/service
ADD . .
RUN ls -lah
RUN go mod download
RUN CGO_LDFLAGS="-static -L/usr/lib/x86_64-linux-gnu/" CGO_ENABLED=1 GOOS=linux go build

FROM alpine
WORKDIR /root
COPY --from=builder /go/src/service /root
