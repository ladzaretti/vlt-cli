package cli

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/spf13/cobra"
)

// vltExportHeader is the CSV header for exported vlt data.
const vltExportHeader = "name,secret,labels"

type ExportError struct {
	Err error
}

func (e *ExportError) Error() string { return "export: " + e.Err.Error() }

func (e *ExportError) Unwrap() error { return e.Err }

type ExportOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions

	output string
	stdout bool
}

var _ genericclioptions.CmdOptions = &ExportOptions{}

// NewExportOptions initializes the options struct.
func NewExportOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *ExportOptions {
	return &ExportOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
	}
}

func (*ExportOptions) Complete() error { return nil }

func (o *ExportOptions) Validate() error {
	if len(o.output) == 0 && !o.stdout {
		return &ExportError{errors.New("either specify an --output path or use --stdout")}
	}

	return nil
}

func (o *ExportOptions) Run(ctx context.Context, _ ...string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = &ExportError{retErr}
			return
		}
	}()

	var out io.Writer

	if len(o.output) > 0 {
		f, err := os.Create(o.output)
		if err != nil {
			return err
		}
		defer func() { //nolint:wsl
			_ = f.Close()
		}()

		out = f
	}

	if o.stdout {
		out = o.Out
	}

	w := csv.NewWriter(out)
	defer w.Flush()

	secrets, err := o.vault.ExportSecrets(ctx)
	if err != nil {
		return err
	}

	if err := w.Write(strings.Split(vltExportHeader, ",")); err != nil {
		return err
	}

	for _, secret := range secrets {
		labels := strings.Join(secret.Labels, ",")
		if err := w.Write([]string{secret.Name, secret.Value, labels}); err != nil {
			return err
		}
	}

	return nil
}

// NewCmdExport creates the export cobra command.
func NewCmdExport(defaults *DefaultVltOptions) *cobra.Command {
	o := NewExportOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export secrets to a file or stdout",
		Long: `Export secrets in CSV format.
	
Use --output to specify a file path or --stdout to print to standard output (unsafe).`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
	cmd.Flags().StringVarP(&o.output, "output", "o", "", "export secrets to the specified file path")
	cmd.Flags().BoolVarP(&o.stdout, "stdout", "", false, "print exported secrets to standard output (unsafe)")

	return cmd
}
