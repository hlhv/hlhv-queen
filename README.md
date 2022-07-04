# hlhv-queen

[![A+ Golang report card.](https://img.shields.io/badge/go%20report-A+-brightgreen.svg?style=flat)](https://goreportcard.com/report/github.com/hlhv/hlhv-queen)

This is the HLHV queen cell. It's job is to route incoming HTTPS requests
to other cells.

## Usage

Running the program will automatically load with the default options, unless
specified.

Run `hlhv --help` for detailed usage information.

## Using Certificates

HLHV is HTTPS only, so a tls key and certificate are required. Their paths can
be specified in the configuration file, and are by default looked for at
`/var/hlhv/cert/key.pem` and `/var/hlhv/cert/cert.pem` respectively.

HLHV uses this cert for both incoming HTTPS connections, and for communication
with cells. Cells rely on public key authentication in order to confirm the
queen cell they are connecting to is legitimate. Therefore, **if you are using a
self-signed certificate, you should create your own certificate authority** and
give the root certificate to connecting cells. Instructions on how to do this
can be found here:

<https://jamielinux.com/docs/openssl-certificate-authority/>

The HLHV configuration tool ![wrench](https://github.com/hlhv/wrench) will
eventually be able to perform this task automatically.

## Configuration

By default, the configuration file for the queen cell is located at
`/etc/hlhv/hlhv.conf`. A custom file can be specified by running the program
with the arguments `hlhv --conf-path /path/to/conf/file`.

The configuration file has a simple syntax:

```
# comment
<key> <value>
<key> <value>
# ...etc
```

Each line of the file is either a comment, or a whitespace-separated key/value
pair. If multiple lines exist that all set the same key, the last one will be
used. Some keys, however, behave as commands, and do not exhibit this behavior.

### Commands

#### `alias <pattern> -> <value>`
Automatically replace domain names in the incoming request that match
the pattern with the specified value. This is mostly useful for aliasing
multiple domains to `@`, which is what cells should normally mount
under. By default, `localhost`, `127.0.0.1`, `::ffff:127.0.0.1`, and
`::1` are all aliased to `@`. By specifying `(fallback)` as the pattern,
it is possible to alias all requests which did not match a preexisting
alias to the specified value. However, use of this should be avoided.

#### `unalias <pattern>`
Remove an alias. This works on the default aliases as well.

### Keys

#### `keyPath`
Specify the TLS key path. Default: `/var/hlhv/cert/key.pem`

#### `certPath`
Specify the TLS certificate path. Default: `/var/hlhv/cert/cert.pem`

#### `connKey`
A bcrypt hash string specifying the passkey that cells will need to send
to the server in order to connect. This has a default value of empty
and not setting it will cause the server to tell you on startup why
exactly doing so is a bad idea.

You can generate a hash to use here with
![this tool](https://github.com/hlhv/wrench).

#### `portHlhv`
An integer specifying the port that the server will listen for new
connections on. Default: `2001`

#### `portHttps`
An integer specifying the port that the server will listen for new
HTTPS requests on. Default: `443`

#### `gardenFreq`
The interval, in seconds, at which excess bands will be closed, freeing
up resources. Default: `120`

#### `maxBandAge`
The maximum time, in seconds, an band can be inactive before it is
closed. Default: `60`

#### `timeout`
The amount of time, in seconds, a cell has to respond to the server.
This is currently only used during the login process. Default: `1`

#### `timeoutReadHeader`
The amount of time, in seconds, an HTTPS client has to send request
headers. Default: `5`

#### `timeoutRead`
The amount of time, in seconds, an HTTPS client has to send the entire
request. Default: `10`

#### `timeoutWrite`
The amount of time, in seconds, the server has to send a response back
to the client. Default: `15`

#### `timeoutIdle`
The amount of time, in seconds, to wait for the next request when
keep-alives are enabled. Default: `120`
