# Docker Download #

Download Docker image from [Docker Hub](https://hub.docker.com/) into a folder using the [Image Manifest V2, Schema 2](https://distribution.github.io/distribution/spec/manifest-v2-2/)

Golang port of [docker-drag/docker_pull.py](https://github.com/NotGlop/docker-drag/blob/master/docker_pull.py) with only stdlib dependencies

Differences from from [docker-drag](https://github.com/NotGlop/docker-drag)
- Use the same access token for all blob layers
- Reuse layerIds from the container config json instead of generating fake-ids

![Sequence Diagram](/doc/img/seq.png "Sequence Diagram")

## Usage ##

The following command line flags are supported

| Flag   | Description                                     | Mandatory | Default Value     |
|--------|-------------------------------------------------|-----------|-------------------|
| `i`    | The `image` to download of the form `image:tag` | Yes       | NA                |
| `out`  | Target `directory` to download the image        | No        | Basename of image |
| `v`    | Print verbose log                               | No        | false             |
| `os`   | Select container operating system               | No        | linux             |
| `arch` | Select container architecture                   | No        | amd64             |
| `h`    | Print help                                      | No        | false             |

### Examples ###

``` sh
docker-download -i swaggerapi/swagger-editor
```

Should download the image with tag `latest` into a folder named `swagger-editor` in the directory from which the command was invoked

```sh
docker-download -i swaggerapi/swagger-editor:v4-latest -out v4-latest
```

Should download the image into a folder named `v4-latest` in the directory from which the command was invoked

``` sh
docker-download -i swaggerapi/swagger-editor:next-v5 -out next-v5 -os linux -arch amd64
```

Should download the tag `next-v5` for the image matching linux/amd64 into `next-v5` folder

### Other Similar Projects ###

- [DockerPull](https://github.com/FT-Fetters/DockerPull) / Java
- [DockerImageSave](https://github.com/jadolg/DockerImageSave) / Golang
- [Docker-downloader](https://github.com/hatamiarash7/Docker-downloader) / Bash & Python
- [Skopeo](https://github.com/containers/skopeo) / Golang - much larger scope
