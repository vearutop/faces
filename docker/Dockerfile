FROM ubuntu:18.04 as builder

RUN apt-get update
RUN apt-get install -y build-essential cmake wget
RUN mkdir /dlib && cd /dlib && wget http://dlib.net/files/dlib-19.24.tar.bz2 && tar xf dlib-19.24.tar.bz2
RUN cd /dlib/dlib-19.24 && mkdir build && cd build && cmake .. && cmake --build . --config Release && make install && rm -rf /dlib
RUN apt-get install -y  libjpeg-dev
RUN wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz && rm -rf go1.21.6.linux-amd64.tar.gz

WORKDIR /app
ADD . .
RUN CGO_LDFLAGS="-static" CGO_ENABLED=1 /usr/local/go/bin/go build .

FROM alpine
WORKDIR /root
ENTRYPOINT ["/bin/faces"]
COPY --from=builder /app/faces /bin/faces
