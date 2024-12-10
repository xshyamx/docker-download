# Docker Download #

Download Docker image from [Docker Hub](https://hub.docker.com/) into a folder using the [Image Manifest V2, Schema 2](https://distribution.github.io/distribution/spec/manifest-v2-2/)

Golang port of [docker_pull.py](https://github.com/NotGlop/docker-drag/blob/master/docker_pull.py)  from [docker-drag](https://github.com/NotGlop/docker-drag) with the following changes

- Use the same access token for all blob layers
- Reuse layerIds from the container config json instead of generating fake-ids

## Usage ##

The following command line flags are supported

| Flag | Description                                     | Mandatory | Default Value     |
|------|-------------------------------------------------|-----------|-------------------|
| `i`  | The `image` to download of the form `image:tag` | Yes       | NA                |
| `o`  | Target `directory` to download the image        | No        | Basename of image |
| `v`  | Print verbose log                               | No        | false             |
| `h`  | Print help                                      | No        | false             |
