package vaulterrors

import "errors"

var (
	ErrVaultFileExists           = errors.New("vault file already exists")
	ErrVaultFileNotFound         = errors.New("vault file does not exist")
	ErrWrongPassword             = errors.New("incorrect vault password")
	ErrEmptyPassword             = errors.New("empty vault password")
	ErrNonInteractiveUnsupported = errors.New("non-interactive input not supported")
	ErrInteractiveLoginDisabled  = errors.New("interactive login is disabled; no session available")
	ErrEmptySecret               = errors.New("secret cannot be empty")
	ErrSearchNoMatch             = errors.New("no match found")
	ErrAmbiguousSecretMatch      = errors.New("ambiguous secret match: multiple secrets match the search criteria")
)
