package cmd

import (
	"context"
	"slices"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"
	"github.com/ladzaretti/vlt-cli/pkg/vlt/store"
)

type SearchableOptions struct {
	*genericclioptions.SearchOptions
}

// search queries the vault for secrets based on the fields
// set in [genericclioptions.SearchOptions].
func (o *SearchableOptions) search(ctx context.Context, vault *vlt.Vault) ([]labeledSecretPair, error) {
	switch {
	case len(o.IDs) > 0:
		return orderedSecretsOrErr(vault.SecretsByIDs(ctx, o.IDs))

	case len(o.Name) > 0 && len(o.Labels) > 0:
		return orderedSecretsOrErr(vault.SecretsByLabelsAndName(ctx, o.Name, o.Labels...))

	case len(o.Name) > 0:
		return orderedSecretsOrErr(vault.SecretsByName(ctx, o.Name))

	case len(o.Labels) > 0:
		return orderedSecretsOrErr(vault.SecretsByLabels(ctx, o.Labels...))

	default:
		return orderedSecretsOrErr(vault.SecretsWithLabels(ctx))
	}
}

type labeledSecretPair struct {
	id     int
	secret store.LabeledSecret
}

func orderedSecretsOrErr(m map[int]store.LabeledSecret, err error) ([]labeledSecretPair, error) {
	if err != nil {
		return nil, err
	}

	return sortByLabelsCount(m), nil
}

// sortByLabelsCount takes a map of labeled secrets and
// returns a slice sorted by descending label count.
func sortByLabelsCount(m map[int]store.LabeledSecret) []labeledSecretPair {
	sorted := make([]labeledSecretPair, 0)
	for id, labeled := range m {
		sorted = append(sorted, labeledSecretPair{id, labeled})
	}

	// Sort in descending order by label count
	slices.SortFunc(sorted, func(a, b labeledSecretPair) int {
		return len(b.secret.Labels) - len(a.secret.Labels)
	})

	return sorted
}
