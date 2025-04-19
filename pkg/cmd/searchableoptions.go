package cmd

import (
	"context"
	"errors"
	"slices"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"
	"github.com/ladzaretti/vlt-cli/pkg/vlt/store"
)

var ErrNoMatch = errors.New("no match")

type SearchableOptions struct {
	*genericclioptions.SearchOptions
}

// search queries the vault for secrets based on the fields
// set in [genericclioptions.SearchOptions].
//
// For any matched secret, it returns all labels associated with it,
// regardless of the filter options used.
// The resulting slice of pairs is ordered by label count in descending order.
func (o *SearchableOptions) search(ctx context.Context, vault *vlt.Vault) ([]labeledSecretPair, error) {
	switch {
	case len(o.IDs) > 0:
		return orderedSecretsOrErr(vault.SecretsByIDs(ctx, o.IDs...))

	case len(o.Name) > 0 && len(o.Labels) > 0:
		return orderedFullSecretsOrErr(ctx, vault, func() ([]labeledSecretPair, error) {
			return orderedSecretsOrErr(vault.SecretsByLabelsAndName(ctx, o.Name, o.Labels...))
		})

	case len(o.Name) > 0:
		return orderedFullSecretsOrErr(ctx, vault, func() ([]labeledSecretPair, error) {
			return orderedSecretsOrErr(vault.SecretsByName(ctx, o.Name))
		})

	case len(o.Labels) > 0:
		return orderedFullSecretsOrErr(ctx, vault, func() ([]labeledSecretPair, error) {
			return orderedSecretsOrErr(vault.SecretsByLabels(ctx, o.Labels...))
		})

	default:
		return orderedSecretsOrErr(vault.SecretsWithLabels(ctx))
	}
}

type labeledSecretPair struct {
	id     int
	secret store.LabeledSecret
}

type matchSecretsFunc func() ([]labeledSecretPair, error)

// orderedFullSecretsOrErr returns full secrets (i.e., with all labels),
// ordered in descending order by the initial matched label count in matchedSecrets.
func orderedFullSecretsOrErr(ctx context.Context, vault *vlt.Vault, matchSecrets matchSecretsFunc) ([]labeledSecretPair, error) {
	orderedMatched, err := matchSecrets()
	if err != nil {
		return nil, err
	}

	if len(orderedMatched) == 0 {
		return nil, nil
	}

	orderedIDs := make([]int, len(orderedMatched))
	for i, pair := range orderedMatched {
		orderedIDs[i] = pair.id
	}

	fullSecrets, err := vault.SecretsByIDs(ctx, orderedIDs...)
	if err != nil {
		return nil, err
	}

	orderedFull := make([]labeledSecretPair, len(fullSecrets))
	for i, id := range orderedIDs {
		orderedFull[i] = labeledSecretPair{id: id, secret: fullSecrets[id]}
	}

	return orderedFull, nil
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
	sorted := make([]labeledSecretPair, 0, len(m))
	for id, labeled := range m {
		sorted = append(sorted, labeledSecretPair{id, labeled})
	}

	// Sort in descending order by label count
	slices.SortFunc(sorted, func(a, b labeledSecretPair) int {
		return len(b.secret.Labels) - len(a.secret.Labels)
	})

	return sorted
}
