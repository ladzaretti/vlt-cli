# Vlt [WIP]
A command-line password manager backed by SQLite.

## TODO

- [ ] Implement all initial subcommands
  - [x] login
  - [ ] logout  (session)
  - [x] create  (alias: new)
  - [x] save    (alias: put)
  - [x] show    (alias: get)
  - [ ] update
    - by id (required), update --secret --name, or interactive, support clipboard and piping (secret only).
  - [ ] labels 
    - [ ] add 
      - by id
    - [ ] remove 
      - by id
  - [x] remove  (alias: rm, delete)
  - [x] find    (alias: list, ls)
- [ ] Add a cryptographic layer
- [ ] Add session support
