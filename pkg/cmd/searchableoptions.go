package cmd

import (
	"context"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"
	"github.com/ladzaretti/vlt-cli/pkg/vlt/store"
)

type SearchableOptions struct {
	*genericclioptions.SearchOptions
}

// Search queries the vault for secrets based on the fields
// set in [genericclioptions.SearchOptions].
func (o *SearchableOptions) Search(ctx context.Context, vault *vlt.Vault) (map[int]store.LabeledSecret, error) {
	switch {
	case len(o.IDs) > 0:
		return vault.SecretsByIDs(ctx, o.IDs)

	case len(o.Name) > 0 && len(o.Labels) > 0:
		return vault.SecretsByLabelsAndName(ctx, o.Name, o.Labels...)

	case len(o.Name) > 0:
		return vault.SecretsByName(ctx, o.Name)

	case len(o.Labels) > 0:
		return vault.SecretsByLabels(ctx, o.Labels...)

	default:
		return vault.SecretsWithLabels(ctx)
	}
}
