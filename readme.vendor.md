# Vendored Patch: modernc.org/sqlite

This vendor copy includes the following patch:

- `type Conn = conn` added to expose the unexported struct.

Running `go mod vendor` will overwrite these changes. Reapply patch with `make vendor-patch`.