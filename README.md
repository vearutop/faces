# faces

[![Time Tracker](https://wakatime.com/badge/github/vearutop/faces.svg)](https://wakatime.com/badge/github/vearutop/faces)
![Code lines](https://sloc.xyz/github/vearutop/faces/?category=code)
![Comments](https://sloc.xyz/github/vearutop/faces/?category=comments)

Face detection HTTP microservice based on [`dlib`](https://github.com/davisking/dlib-models).

## Usage

```
./faces -h
Usage of ./faces:
  -listen string
        listen address (default "localhost:8011")
```

```
./faces 
2024/01/15 23:44:22 recognizer init 424.357089ms
2024/01/15 23:44:22 http://localhost:8011/docs
```

This repo contains models, that were created by `Davis King <https://github.com/davisking/dlib-models>`__ and are 
licensed in the public domain or under CC0 1.0 Universal. See [LICENSE](./LICENSE).
