# Vlt [WIP]
A command-line password manager backed by SQLite.

## TODO

- [ ] Implement all initial subcommands
  - [x] login
  - [ ] logout  (session)
  - [x] create  (alias: new)
  - [x] save    (alias: put)
  - [x] show    (alias: get)
  - [ ] update (requires --id)
    - [x] secret (requires --id, supports CLI args or interactive mode)
  - [x] remove  (alias: rm, delete)
  - [x] find    (alias: list, ls)
  - [x] config
    - [x] generate
    - [x] validate
  - [ ] import
    - firefox
    - chrome
  - [x] generate (alias: rand, gen)
- [ ] Add a cryptographic layer
- [ ] Add session support
