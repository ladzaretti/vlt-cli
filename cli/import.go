package cli

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/util"

	"github.com/spf13/cobra"
)

type ImportError struct {
	Err error
}

func (e *ImportError) Error() string { return "import: " + e.Err.Error() }

func (e *ImportError) Unwrap() error { return e.Err }

const (
	// firefoxHeader defines the expected CSV header for exported Firefox passwords.
	firefoxHeader = "url,username,password,httpRealm,formActionOrigin,guid,timeCreated,timeLastUsed,timePasswordChanged"

	// chromiumHeader defines the expected CSV header for exported Chromium passwords.
	chromiumHeader = "name,url,username,password,note"
)

var (
	// firefoxImporter is a custom password importer for exported Firefox password data.
	firefoxImporter = CustomImporter{
		NameIndex:    util.Ptr(1),
		SecretIndex:  util.Ptr(2),
		LabelIndexes: []int{0, 3, 4},
	}

	// chromiumImporter is a custom password importer for exported Chromium password data.
	chromiumImporter = CustomImporter{
		NameIndex:    util.Ptr(2),
		SecretIndex:  util.Ptr(3),
		LabelIndexes: []int{0, 1, 4},
	}

	// vltImporter is a password importer for exported vlt password data.
	vltImporter = VltImporter{}
)

type VltImporter struct{}

var _ Importer = VltImporter{}

func (VltImporter) validate(record []string) error {
	if len(record) != 3 {
		return &ImportError{errors.New("expected 3 fields per record for vlt csv record")}
	}

	return nil
}

// convert converts a CSV record into a secret.
//
// It assumes that the input record has already been validated, so it may panic on
// out-of-bounds access or invalid hex data.
func (VltImporter) convert(record []string) secret {
	s, err := hex.DecodeString(record[1])
	if err != nil {
		panic(err)
	}

	return secret{
		name:   record[0],
		secret: s,
		labels: strings.Split(record[2], ","),
	}
}

type secret struct {
	name   string
	secret []byte
	labels []string
}

type Importer interface {
	convert(record []string) secret
	validate(record []string) error
}

// CustomImporter defines custom column indexes used to extract fields from a CSV row.
type CustomImporter struct {
	NameIndex    *int  `json:"name,omitempty"`   // NameIndex is the index of the name column.
	SecretIndex  *int  `json:"secret,omitempty"` // SecretIndex is the index of the secret column.
	LabelIndexes []int `json:"labels,omitempty"` // LabelIndexes are the indexes of the label columns.
}

var _ Importer = CustomImporter{}

func (ic CustomImporter) validate(record []string) error {
	if ic.NameIndex == nil {
		return errors.New("name index is not set")
	}

	if ic.SecretIndex == nil {
		return errors.New("secret index is not set")
	}

	if *ic.NameIndex >= len(record) {
		return fmt.Errorf("name index %d is out of range (record has %d columns)", *ic.NameIndex, len(record))
	}

	if *ic.SecretIndex >= len(record) {
		return fmt.Errorf("secret index %d is out of range (record has %d columns)", *ic.SecretIndex, len(record))
	}

	for _, index := range ic.LabelIndexes {
		if index >= len(record) {
			return fmt.Errorf("label index %d is out of range (record has %d columns)", index, len(record))
		}
	}

	return nil
}

func (ic CustomImporter) convert(record []string) secret {
	// safe to dereference since validate is expected to run first.
	s := secret{
		name:   record[*ic.NameIndex],
		secret: []byte(record[*ic.SecretIndex]),
		labels: make([]string, 0, len(ic.LabelIndexes)),
	}

	for _, labelIndex := range ic.LabelIndexes {
		label := record[labelIndex]
		if len(label) > 0 {
			s.labels = append(s.labels, label)
		}
	}

	return s
}

func (ic CustomImporter) String() string {
	name := "nil"
	if ic.NameIndex != nil {
		name = strconv.Itoa(*ic.NameIndex)
	}

	secret := "nil"
	if ic.SecretIndex != nil {
		secret = strconv.Itoa(*ic.SecretIndex)
	}

	return fmt.Sprintf(`{"name": %s, "secret": %s, "labels": %v}`, name, secret, ic.LabelIndexes)
}

type ImportOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions

	indexes string

	importConfig CustomImporter
}

var _ genericclioptions.CmdOptions = &ImportOptions{}

// NewImportOptions initializes the options struct.
func NewImportOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *ImportOptions {
	return &ImportOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
	}
}

func (o *ImportOptions) Complete() error {
	if len(o.indexes) > 0 {
		if err := json.Unmarshal([]byte(o.indexes), &o.importConfig); err != nil {
			return &ImportError{err}
		}
	}

	return nil
}

func (*ImportOptions) Validate() error { return nil }

func (o *ImportOptions) Run(ctx context.Context, files ...string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = &ImportError{retErr}
			return
		}
	}()

	switch {
	case o.StdinIsPiped && len(files) > 0:
		return errors.New("cannot import from both stdin and file")

	case o.StdinIsPiped:
		o.Infof("importing secrets from stdin")
		return o.importSecrets(ctx, o.In)

	case len(files) == 1:
		return o.importFromFile(ctx, files[0])

	case len(files) > 1:
		return errors.New("only one input file can be imported at a time")

	default:
		return errors.New("no input source provided (stdin or file)")
	}
}

func (o *ImportOptions) importSecrets(ctx context.Context, in io.Reader) error {
	r := csv.NewReader(in)

	header, err := r.Read()
	if err != nil {
		return err
	}

	importer := o.importerForHeader(strings.Join(header, ","))
	if err := importer.validate(header); err != nil {
		return err
	}

	i := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		s := importer.convert(record)

		if _, err := o.vault.InsertNewSecret(ctx, s.name, s.secret, s.labels); err != nil {
			return err
		}

		clear(record)
		clear(s.secret)

		i++
	}

	o.Infof("successfully imported %d records\n", i)

	return nil
}

func (o *ImportOptions) importFromFile(ctx context.Context, name string) error {
	f, err := os.Open(filepath.Clean(name))
	if err != nil {
		return err
	}
	defer func() { //nolint:wsl
		_ = f.Close()
	}()

	o.Infof("importing secrets from: %q\n", name)

	return o.importSecrets(ctx, f)
}

//nolint:ireturn
func (o *ImportOptions) importerForHeader(header string) Importer {
	switch header {
	case firefoxHeader:
		o.Infof("firefox export file detected\n")
		return firefoxImporter

	case chromiumHeader:
		o.Infof("chromium export file detected\n")
		return chromiumImporter

	case vltExportHeader:
		o.Infof("vlt export file detected\n")
		return vltImporter

	default:
		o.Debugf("using custom import config: %s\n", o.importConfig)
		return o.importConfig
	}
}

// NewCmdImport creates the import cobra command.
func NewCmdImport(defaults *DefaultVltOptions) *cobra.Command {
	o := NewImportOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
	)

	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import secrets from file (supports Firefox, Chromium, and custom formats)",
		Args:  cobra.ArbitraryArgs,
		Long: `Import secrets into the vault from a CSV file.

The input must be a CSV file with at least two columns: one for the secret's name and one for its value (e.g., password). 
Additional columns can be used for optional labels.

Use the --indexes flag to specify how to extract each field. 
Indexes are zero-based and refer to column positions in the header row.

Firefox and Chromium-based CSV files are auto-detected for import and do not require manual index specification.
`,
		Example: `  # Import secrets from a file (format is auto-detected if compatible)
  vlt import passwords.csv
  
  # Import from custom CSV data using a column mapping
  echo -e "password,username,label_1,label_2\npass,some_username,meta1,meta2" | \
    vlt import \
        --indexes '{"name":1,"secret":0,"labels":[2,3]}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, args...))
		},
	}

	cmd.Flags().StringVarP(&o.indexes, "indexes", "i", "", "json with column indexes (e.g., '{\"name\":0,\"secret\":1,\"labels\":[2]}')")

	return cmd
}
