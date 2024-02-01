# faces

[![wakatime](https://wakatime.com/badge/user/d4d32a01-9dcc-43f3-96a3-fe3be55e75fd/project/018b8b57-d925-41b0-8f56-b65f8c46b532.svg)](https://wakatime.com/badge/user/d4d32a01-9dcc-43f3-96a3-fe3be55e75fd/project/018b8b57-d925-41b0-8f56-b65f8c46b532)
![Code lines](https://sloc.xyz/github/vearutop/faces/?category=code)
![Comments](https://sloc.xyz/github/vearutop/faces/?category=comments)

Face detection HTTP microservice based on [`dlib`](https://github.com/davisking/dlib-models).

## Installation

Portable statically-linked binary for Linux AMD64 is available
on [releases](https://github.com/vearutop/faces/releases).

```
wget https://github.com/vearutop/faces/releases/latest/download/linux_amd64.tar.gz && tar xf linux_amd64.tar.gz && rm linux_amd64.tar.gz
./faces -h
```

It is also available as docker image.

```
docker run --rm -p 8011:80 vearutop/faces
```

If you want to build the app from source, please follow the instructions on
[dependencies setup](https://github.com/Kagami/go-face?tab=readme-ov-file#requirements).

## Usage

```
./faces -h
Usage of ./faces:
  -listen string
        listen address (default "localhost:8011")
```

Start server.

```
./faces
2024/01/15 23:44:22 recognizer init 424.357089ms
2024/01/15 23:44:22 http://localhost:80/docs
```

Send request.

```
curl -X 'POST' \
  'http://localhost:8011/image' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'image=@faces.jpg;type=image/jpeg'
```

```json
{
  "elapsedSec": 2.373028184,
  "found": 4,
  "faces": [
    {
      "Rectangle": {
        "Min": {
          "X": 584,
          "Y": 1228
        },
        "Max": {
          "X": 1029,
          "Y": 1673
        }
      },
      "Descriptor": [
        -0.122200325,
        0.10511437,
        0.05358115,
        0.011272355,
        -0.09460048,
        "............. cut here ..........."
      ]
    }
  ]
}
```

This repo contains models, that were created by `Davis King <https://github.com/davisking/dlib-models>`__ and are
licensed in the public domain or under CC0 1.0 Universal. See [LICENSE](./LICENSE).
