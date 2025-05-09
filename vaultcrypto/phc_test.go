package vaultcrypto_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ladzaretti/vlt-cli/vaultcrypto"
)

var b64 = base64.StdEncoding.WithPadding(base64.NoPadding)

func TestArgon2idPHC_String(t *testing.T) {
	tests := []struct {
		name string
		phc  vaultcrypto.Argon2idPHC
		want string
	}{
		{
			name: "with hash",
			phc: vaultcrypto.Argon2idPHC{
				Version: 19,
				Argon2Params: vaultcrypto.Argon2Params{
					Memory:      64 * 1024,
					Time:        3,
					Parallelism: 4,
				},
				Salt: []byte("salt"),
				Hash: []byte("hash"),
			},
			want: fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s", b64.EncodeToString([]byte("salt")), b64.EncodeToString([]byte("hash"))),
		},
		{
			name: "without hash",
			phc: vaultcrypto.Argon2idPHC{
				Version: 19,
				Argon2Params: vaultcrypto.Argon2Params{
					Memory:      32 * 1024,
					Time:        2,
					Parallelism: 2,
				},
				Salt: []byte("salt"),
			},
			want: "$argon2id$v=19$m=32768,t=2,p=2$" + b64.EncodeToString([]byte("salt")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.phc.String(); got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

//nolint:revive
func TestDecodeAragon2idPHC(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    vaultcrypto.Argon2idPHC
		wantErr bool
	}{
		{
			name:  "valid with hash",
			input: fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s", b64.EncodeToString([]byte("salt")), b64.EncodeToString([]byte("hash"))),
			want: vaultcrypto.Argon2idPHC{
				Version: 19,
				Argon2Params: vaultcrypto.Argon2Params{
					Memory:      65536,
					Time:        3,
					Parallelism: 4,
				},
				Salt: []byte("salt"),
				Hash: []byte("hash"),
			},
			wantErr: false,
		},
		{
			name:  "valid without hash",
			input: "$argon2id$v=19$m=65536,t=3,p=4$" + b64.EncodeToString([]byte("salt")),
			want: vaultcrypto.Argon2idPHC{
				Version: 19,
				Argon2Params: vaultcrypto.Argon2Params{
					Memory:      65536,
					Time:        3,
					Parallelism: 4,
				},
				Salt: []byte("salt"),
				Hash: nil,
			},
			wantErr: false,
		},
		{
			name:    "invalid prefix",
			input:   "$argon2i$v=19$m=65536,t=3,p=4$" + b64.EncodeToString([]byte("salt")),
			wantErr: true,
		},
		{
			name:    "invalid base64 salt",
			input:   "$argon2id$v=19$m=65536,t=3,p=4$!!invalid!!",
			wantErr: true,
		},
		{
			name:    "missing fields",
			input:   "$argon2id$v=19$m=65536,t=3,p=4",
			wantErr: true,
		},
		{
			name:    "unsupported version",
			input:   "$argon2i$v=10$m=65536,t=3,p=4$" + b64.EncodeToString([]byte("salt")),
			wantErr: true,
		},
		{
			name:    "malformed params",
			input:   "$argon2id$v=19$m=bad,t=3,p=4$c29tZXNhbHQ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vaultcrypto.DecodeAragon2idPHC(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if got.Argon2Params != tt.want.Argon2Params {
				t.Errorf("params mismatch: got %+v, want %+v", got.Argon2Params, tt.want.Argon2Params)
			}

			if !bytes.Equal(got.Salt, tt.want.Salt) {
				t.Errorf("salt mismatch: got=%q, want=%q", got.Salt, tt.want.Salt)
			}

			if !bytes.Equal(got.Hash, tt.want.Hash) {
				t.Errorf("hash mismatch: got=%q, want=%q", got.Hash, tt.want.Hash)
			}
		})
	}
}
