# Vlt [WIP]
A command-line password manager backed by SQLite.

## TODO

- [ ] Implement all initial subcommands
  - [x] create
  - [x] login
  - [ ] logout (session)
  - [x] save
  - [ ] update
  - [ ] show
    - by --label strings, --name string, --id. + all other output related flags.
    - only accept a single match. error otherwise.
    - print table for matches with more than one matching secret.
  - [x] remove
  - [x] find
- [ ] Add a cryptographic layer
- [ ] Add session support
