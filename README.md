# hlhv

HLHV is a modular, hot-pluggable HTTPS server. It is designed to simplify the
process of creating sites that have differently behaving sections.

It redirects incoming requests to sub-servers called cells which mount
themselves on different parts of a URL. Cells connect to a central server
(called the queen cell) over TLS sockets, which allows the entire system to be
distributed across multiple machines or containers. Since cells connect to the
queen and not the other way around, the queen is the only server that needs to
have open ports.

Cells can mount on different URL paths, subdomains, etc. Upon receiving an HTTPS
request, the queen will direct it to the cell with the most specific matching
mount. For example, if there is a cell mounted on `@/photos`, and another on
`@/photos/dinosaurs`, and a request comes in for `@/photos/dinosaurs/debian.webp`,
it will be directed to the cell mounted on `@/photos/dinosaurs`. The cell would
receive the entire request path, and send the correct file to the queen, where
it would be sent back to the end user.
