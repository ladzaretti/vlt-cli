# Vlt [WIP]
A command-line password manager backed by SQLite.

## TODO

- [ ] Implement all initial subcommands
  - [x] login
  - [ ] logout (session)
  - [x] new
  - [x] put     (alias: save)
  - [ ] get     (alias: show)
  - [ ] update
    - by --label strings, --name string, --id. + all other output related flags.
    - only accept a single match. error otherwise.
    - print table for matches with more than one matching secret.
  - [x] delete  (alias: remove, rm)
  - [x] find    (alias: list, ls)
- [ ] Add a cryptographic layer
- [ ] Add session support
