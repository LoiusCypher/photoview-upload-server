photoview-upload-server
=======================

This work is based on https://github.com/mayth/go-simple-upload-server a Simple HTTP server to save artifacts

It was modified and serves now as an (PDF) upload and administration API that converts the documents to
JPEGs that can be viewed through photoview. For this purpose the uploaded PDFs are needed only to generate
the JPEGs and are removed afterwards. Only an SHA sum of every PDF file is recoreded to enable the client SW
if any PDF was modified since last upload.

- [Usage](#usage)
- [Timeouts](#timeouts)
- [Testing](#testing)
- [API](#api)
  - [`GET /list/?name=path`](#get-listnamepath)
  - [`OPTIONS /list/`](#options-list)
  - [`POST /upload/`](#post-file)
  - [`GET /file/?name=path`](#get-filenamepath`)
  - [`DELETE /file/?name=path`](#delete-file)
  - [`OPTIONS /file/`](#options-file)


## Usage

```
  -addr string
        address to listen
  -config string
        path to config file
  -picture_root string
        path to root of directory tree where JPEGs will be stored
  -upload_root string
        path to directory where uploaded PDFs are temporarily stored until they are converted to JPEGs
  -enable_auth
        enable authentication
  -enable_cors
        enable CORS header
  -file_naming_strategy string
        File naming strategy
  -max_upload_size int
        max upload size in bytes
  -read_only_tokens value
        comma separated list of read only tokens
  -read_timeout duration
        read timeout. zero or negative value means no timeout. can be suffixed by the time units 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h' (e.g. '1s', '500ms'). If no suffix is provided, it is interpreted as seconds.
  -read_write_tokens value
        comma separated list of read write tokens
  -shutdown_timeout int
        graceful shutdown timeout in milliseconds
  -write_timeout duration
        write timeout. zero or negative value means no timeout. same format as read_timeout.
```

Configurations via the arguments take precedence over those came from the config file.

`max_upload_size` limits the size of the uploaded file in bytes (default: 1 MiB).
Multipart parsing is also limited to `max_upload_size` plus 1 MiB for multipart
framing, metadata, and other parts. Uploads exceeding either limit return
`413 Request Entity Too Large` before creating or overwriting a destination file.

## Authentication

This server does not require authentication by default. Anyone who can access the server can get/upload files.

The server implements a simple authentication mechanism using tokens.

1. Configure the server to enable authentication: `"enable_auth": true` or `-enable_auth=true`.
2. Prepare tokens. Any string value is valid as a token.
3. Add them to the configuration. There are two keys in configuration: `read_only_tokens` and `read_write_tokens`.
   If authentication is enabled but no tokens provided, the server generates a read-only token and a read-write token on its starting up.
4. Request with the token. Add Authorization header with value `Bearer <TOKEN>` or `token=<TOKEN>` to the query parameter. Authorization header takes precedence.

| Token Type |          Allowed Operations         |
| ---------- | ----------------------------------- |
| read-only  | `GET`, `HEAD`                       |
| read-write | `POST` in addition to read-only ops |

Note that `OPTIONS` is always allowed without authentication.

Authentication is failed when:

* A request has no tokens.
* A request has a token but not registered to the server.
* A request has a token but not allowed to the requested operation.

In these cases, the server respond with `401 Unauthorized` with body like as: `{"ok": false, "error": "unauthorized"}`.

No one can request write operations if you configures the server with read-only tokens only.
As a result, the server operates like read-only mode.

## TLS

v1 has TLS support but I decided to omit it from v2.

Please consider using a reverse proxy like nginx.

## Timeouts

There are 2 timeout configurations: read and write.
The terms "read" and "write" are from the server's perspective. From clients, they are "upload" (`PUT`) and "get info" (`GET`) respectively.

Read timeout (`-read_timeout`) is the maximum duration for the server reading the request.
Clients should finish sending request headers and the entire content within this timeout.
This is set to 15 seconds by default.

Write timeout (`-write_timeout`) is the maximum duration for the server writing the response.
Clients should finish downloading the content within this timeout.
This timeout is not set by default. Before v2.1.0, this is set to 15 seconds.

Please consider changing these timeout if:

* the server or the clients are in a low-bandwidth network.
* you are working with large files.

Note that a longer timeout will result in more connections being maintained.

## Testing

To run all tests, just run `go test` as usual:

```
$ go test ./...
```

This includes end-to-end tests. By default, the server with on-memory FileSystem is created and it starts listening on
the port chosen randomly. You can control this behavior by setting the environment variables.

The workflow starts with a local file that is uploaded and then converted from PDF to JPG by ghostscript. The uploaded
file is then removed and its SHA is recorded in an SqLite database.
Sqlite and ghostscript are not prepared for virtual filesystem, therefore the tests use temp files for the database location,
the uploaded file and the final JPEG files.
f the test completes all files are cleaned up.

If `TEST_TARGET_ADDR-${HOST}:${PORT}` is set, the test program doesn't start a local test server and sends requests to
`http://${HOST}:${PORT}`. Note that the target server's picture root should be cleared prior to testing.

This repository has `docker-compose.e2e.yml` to run the E2E test. To run tests using this:

```
$ docker compose -f docker-compose.e2e.yml run --rm test
$ docker compose -f docker-compose.e2e.yml down --rmi local --volumes
```

## API

### `GET /list?name=path`
<<<<<<< HEAD
Uploads a new file. The name of the local (= server-side) file is taken from the uploading file.

#### Request

Content-Type
: `multipart/form-data`

Parameters:

|    Name     | Required? |   Type    |                         Description                          | Default |
| ----------- | :-------: | --------- | ------------------------------------------------------------ | ------- |
| `file`      |     x     | Form Data | A content of the file.                                       |         |
| `overwrite` |           | `boolean` | Allow overwriting the existing file on the server if `true`. | `false` |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `ok`   | `boolean` | `true` if successful.                   |
| `path` | `string`  | A path to access this file in this API. |

##### On Failure

|           StatusCode           |                                              When                                              |
| ------------------------------ | ---------------------------------------------------------------------------------------------- |
| `413 Request Entity Too Large` | The file or multipart body exceeds the upload limits.                                          |
| `409 Conflict`                 | There is the file whose name is the same as the uploading file and overwriting is not allowed. |

#### Example

```
$ echo 'Hello, world!' > sample.txt
$ curl -Ffile=@sample.txt http://localhost:25478/upload
{"ok":true,"path":"/files/sample.txt"}
```

```
$ cat $DOCROOT/sample.txt
Hello, world!
```

### `PUT /files/:path`

Uploads a file. The original file name is ignored and the name is taken from the path in the request URL.

#### Parameters

|    Name     | Required? |   Type    |                    Description                     | Default |
| ----------- | :-------: | --------- | -------------------------------------------------- | ------- |
| `:path`     |     x     | `string`  | Path to the file.                                  |         |
| `file`      |     x     | Form Data | A content of the file.                             |         |
| `overwrite` |           | `boolean` | Allow overwriting the existing file on the server. | `false` |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `ok`   | `boolean` | `true` if successful.                   |
| `path` | `string`  | A path to access this file in this API. |

##### On Failure

|           StatusCode           |                                              When                                              |
| ------------------------------ | ---------------------------------------------------------------------------------------------- |
| `413 Request Entity Too Large` | The file or multipart body exceeds the upload limits.                                          |
| `409 Conflict`                 | There is the file whose name is the same as the uploading file and overwriting is not allowed. |

#### Example

```
$ curl -XPUT -Ffile=@sample.txt "http://localhost:25478/files/foobar.txt"
{"ok":true,"path":"/files/foobar.txt"}

$ cat $DOCROOT/foobar.txt
Hello, world!
```

### `PUT /file?name=path`

Uploads a file. The original file name is ignored and the name is taken from the path in the request URL.

#### Parameters

|    Name     | Required? |   Type    |                    Description                     | Default |
| ----------- | :-------: | --------- | -------------------------------------------------- | ------- |
| `path`      |     x     | `string`  | Path to the file.                                  |         |
| `file`      |     x     | Form Data | Content of the file.                               |         |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `path` | `string`  | A path to access this file in this API. |
| `sha`  | `string`  | Sha256 of the uploaded file contents.   |

##### On Failure

|   StatusCode   |                                              When                                              |
| -------------- | ---------------------------------------------------------------------------------------------- |
|                |                                                                                                |

#### Example

```
$ curl -XPUT -Ffile=@examples/sample.txt --url-query "path=my files/foobar.txt" "http://localhost:25478/file"
{"path":"my files/foobar.txt","sha":"dbc08b031f67a6a5bf77a911a02542d91accb466750276452637ab050994547c"}

### `GET /file?name=path`

Check if file was uploaded before and get its sha256 sum if so.

#### Request

Parameters:

|  Name  | Required? |   Type   |     Description     | Default |
| ------ | :-------: | -------- | ------------------- | ------- |
| `path` |     x     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `200 OK`

Content-Type
: `application/json`

Body

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `path` | `string`  | A path to access this file in this API. |
| `sha`  | `string`  | Sha256 of the uploaded file contents.   |

##### On Failure

Content-Type
: `application/json`

|   StatusCode    |          When          |
| --------------- | ---------------------- |
| `404 Not Found` | There is no such file. |

#### Example

```
$ curl --url-query "name=my files/foobar.txt" "http://localhost:25478/file"
{"path":"my files/foobar.txt","sha":"dbc08b031f67a6a5bf77a911a02542d91accb466750276452637ab050994547c"}

### `DELETE /file?name=path`

Deletes a file. The name is taken from the path in the request URL.

#### Parameters

|    Name     | Required? |   Type    |                    Description                     | Default |
| ----------- | :-------: | --------- | -------------------------------------------------- | ------- |
| `path`      |     x     | `string`  | Path to the file.                                  |         |

#### Response

##### On Successful

Status Code
: `202 Accepted`

Content-Type
: `application/json`

Body:

|  Name  |   Type    |               Description               |
| ------ | --------- | --------------------------------------- |
| `path` | `string`  | A path to access this file in this API. |

##### On Failure

|   StatusCode    |            When             |
| --------------- | --------------------------- |
| `404 Not Found` | No such file on the server. |

#### Example

```
$ curl -X DELETE --url-query "path=my files/foobar.txt" "http://localhost:25478/file"
{"path":"my files/foobar.txt"}

### `OPTIONS /file/`
### `OPTIONS /list/`

CORS preflight request.

#### Request

Parameters:

|  Name  | Required? |   Type   |     Description     | Default |
| ------ | :-------: | -------- | ------------------- | ------- |
| `path` |     x     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `204 No Content`

##### On Failure

#### Example

TODO

#### Notes

* Requests using `*` as a path, like as `OPTIONS * HTTP/1.1`, are not supported.
* On sending `OPTIONS` request, `token` parameter is not required.
* For `/files/:path` request, server replies "204 No Content" even if the specified file does not exist.
