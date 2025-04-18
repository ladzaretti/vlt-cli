# Vlt [WIP]
A command-line password manager backed by SQLite.

## TODO

- [ ] Implement all initial subcommands
  - [x] create
  - [x] login
  - [x] save
  - [ ] show
    - by --label strings, --name string, --id. + all other output related flags.
  - [ ] remove
    - by --label strings, --name string, --id.
  - [x] find
    - by --label strings, --name string, --id. print to stdout.
- [ ] Add a cryptographic layer
- [ ] Add session support

searching by labels is ORed.
searching by name and labels, return the intersection between the two queries.