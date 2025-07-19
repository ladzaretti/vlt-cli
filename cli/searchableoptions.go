package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultdb"
)

// SearchableOptions provides common filtering parameters and methods
// used by CLI commands for querying secrets.
type SearchableOptions struct {
	ID       int
	IDs      []int
	Name     string
	Labels   []string
	Wildcard string
}

type Filter int

const (
	_ Filter = iota
	FilterByID
	FilterByName
	FilterByLabels
)

var help = map[Filter]string{
	FilterByID:     "filter by id",
	FilterByName:   "filter by name",
	FilterByLabels: "filter by label",
}

func (u Filter) Help() string {
	if s, ok := help[u]; ok {
		return s
	}

	return "unknown usage"
}

var _ genericclioptions.BaseOptions = &SearchableOptions{}

type SearchableOptionsOpt func(*SearchableOptions)

func NewSearchableOptions(opts ...SearchableOptionsOpt) *SearchableOptions {
	o := &SearchableOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (*SearchableOptions) Complete() error { return nil }

func (*SearchableOptions) Validate() error { return nil }

func (o *SearchableOptions) WildcardFrom(args []string) {
	if len(args) > 0 {
		o.Wildcard = args[0]
	}
}

// search queries the vault for secrets based on the fields
// set in [genericclioptions.SearchOptions].
//
// For any matched secret, it returns all labels associated with it,
// regardless of the filter options used.
func (o *SearchableOptions) search(ctx context.Context, vault *vault.Vault) ([]secretWithLabels, error) {
	if o.ID > 0 {
		return retrieveSortedByID(func() (map[int]vaultdb.SecretWithLabels, error) {
			return vault.SecretsByIDs(ctx, o.ID)
		})
	}

	if len(o.IDs) > 0 {
		return retrieveSortedByID(func() (map[int]vaultdb.SecretWithLabels, error) {
			return vault.SecretsByIDs(ctx, o.IDs...)
		})
	}

	retrieveSecretsFunc := func() (map[int]vaultdb.SecretWithLabels, error) {
		return vault.FilterSecrets(ctx, o.Wildcard, o.Name, o.Labels)
	}

	if len(o.Labels) > 0 || len(o.Wildcard) > 0 {
		return retrieveSortedByMatch(ctx, vault, retrieveSecretsFunc)
	}

	return retrieveSortedByID(retrieveSecretsFunc)
}

type secretWithLabels struct {
	id     int
	name   string
	labels []string
}

type retrieveSecretsFunc func() (map[int]vaultdb.SecretWithLabels, error)

// retrieveSortedByID returns secrets with all their labels, ordered by id value.
func retrieveSortedByID(retrieveSecretsFunc retrieveSecretsFunc) ([]secretWithLabels, error) {
	secrets, err := retrieveSecretsFunc()
	if err != nil {
		return nil, err
	}

	sortedByID := secretsMapToSlice(secrets)
	slices.SortFunc(sortedByID, func(a, b secretWithLabels) int {
		return b.id - a.id
	})

	return sortedByID, nil
}

// retrieveSortedByMatch returns secrets with all their labels, ordered in
// descending order by the number of labels initially matched by retrieveMatchingFunc.
//
// retrieveMatchingFunc typically returns secrets containing only the labels
// that match the applied filter.
func retrieveSortedByMatch(ctx context.Context, vault *vault.Vault, retrieveSecretsFunc retrieveSecretsFunc) ([]secretWithLabels, error) {
	matchingSecrets, err := retrieveSecretsFunc()
	if err != nil {
		return nil, err
	}

	if len(matchingSecrets) == 0 {
		return nil, nil
	}

	// sort in descending order by label count
	sortedByLabelsCount := secretsMapToSlice(matchingSecrets)
	slices.SortFunc(sortedByLabelsCount, func(a, b secretWithLabels) int {
		// desc by label count
		if lenA, lenB := len(a.labels), len(b.labels); lenA != lenB {
			return lenB - lenA
		}

		// tie break: desc by id
		return b.id - a.id
	})

	sortedIDs := make([]int, len(sortedByLabelsCount))
	for i, secret := range sortedByLabelsCount {
		sortedIDs[i] = secret.id
	}

	secrets, err := vault.SecretsByIDs(ctx, sortedIDs...)
	if err != nil {
		return nil, err
	}

	sortedSecrets := make([]secretWithLabels, len(secrets))
	for i, id := range sortedIDs {
		sortedSecrets[i] = secretWithLabels{
			id:     id,
			name:   secrets[id].Name,
			labels: secrets[id].Labels,
		}
	}

	return sortedSecrets, nil
}

func secretsMapToSlice(m map[int]vaultdb.SecretWithLabels) []secretWithLabels {
	sorted := make([]secretWithLabels, 0, len(m))
	for id, labeled := range m {
		l := secretWithLabels{
			id:     id,
			name:   labeled.Name,
			labels: labeled.Labels,
		}
		sorted = append(sorted, l)
	}

	return sorted
}

func extractIDs(secrets []secretWithLabels) []int {
	ids := make([]int, len(secrets))
	for i, s := range secrets {
		ids[i] = s.id
	}

	return ids
}

func printTable(w io.Writer, markedLabeledSecrets []secretWithLabels) {
	tw := tabwriter.NewWriter(w, 0, 0, 5, ' ', 0)
	defer func() { _ = tw.Flush() }()

	fmt.Fprintln(tw, "ID\tNAME\tLABELS")

	for _, marked := range markedLabeledSecrets {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", marked.id, marked.name, strings.Join(marked.labels, ","))
	}

	fmt.Fprintln(tw) // add padding
}
