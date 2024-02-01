FROM ubuntu:22.04 as builder

RUN apt-get update
RUN apt-get install -y build-essential cmake curl
RUN curl -sLO https://go.dev/dl/go1.21.6.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz && rm -rf go1.21.6.linux-amd64.tar.gz
RUN mkdir /dlib && cd /dlib && curl -sLO http://dlib.net/files/dlib-19.24.tar.bz2 && tar xf dlib-19.24.tar.bz2
RUN cd /dlib/dlib-19.24 && mkdir build && cd build && cmake .. && cmake --build . --config Release && make install && rm -rf /dlib

# Missing header file.
RUN cp /usr/local/include/dlib/external/libjpeg/*.h /usr/include/

# Building app.
WORKDIR /app
ADD . .
RUN CGO_LDFLAGS="-static" /usr/local/go/bin/go build -tags static .

# Exporting minimal docker image with pre-built binary.
FROM alpine
WORKDIR /root
CMD ["/bin/faces", "-listen", "0.0.0.0:80"]
COPY --from=builder /app/faces /bin/faces
