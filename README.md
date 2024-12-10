# Docker Download #

Download Docker image from [Docker Hub](https://hub.docker.com/) into a folder using the [Image Manifest V2, Schema 2](https://distribution.github.io/distribution/spec/manifest-v2-2/)

Golang port of [docker-drag/docker_pull.py](https://github.com/NotGlop/docker-drag/blob/master/docker_pull.py) with only stdlib dependencies

Differences from from [docker-drag](https://github.com/NotGlop/docker-drag)
- Use the same access token for all blob layers
- Reuse layerIds from the container config json instead of generating fake-ids

![Sequence Diagram](/doc/img/seq.png "Sequence Diagram")

## Usage ##

The following command line flags are supported

| Flag | Description                                     | Mandatory | Default Value     |
|------|-------------------------------------------------|-----------|-------------------|
| `i`  | The `image` to download of the form `image:tag` | Yes       | NA                |
| `o`  | Target `directory` to download the image        | No        | Basename of image |
| `v`  | Print verbose log                               | No        | false             |
| `h`  | Print help                                      | No        | false             |

### Examples ###

``` sh
docker-download -i swagger-api/swagger-editor:v4-latest
```

Should download the image into a folder named `swagger-editor` in the directory from which the command was invoked

```sh
docker-download -i swagger-api/swagger-editor:v4-latest -o v4-latest
```

Should download the image into a folder named `v4-latest` in the directory from which the command was invoked
