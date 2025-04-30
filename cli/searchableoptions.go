package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"text/tabwriter"

	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"
	"github.com/ladzaretti/vlt-cli/vlt/store"
)

var ErrNoSearchArgs = errors.New("no search criteria provided; specify at least one of --id, --label, or --name")

type SearchableOptions struct {
	*genericclioptions.SearchOptions
}

// search queries the vault for secrets based on the fields
// set in [genericclioptions.SearchOptions].
//
// For any matched secret, it returns all labels associated with it,
// regardless of the filter options used.
func (o *SearchableOptions) search(ctx context.Context, vault *vlt.Vault) ([]secretWithMarkedLabels, error) {
	switch {
	case len(o.IDs) > 0:
		return sortAndUnmarkSecrets(func() (map[int]store.SecretWithLabels, error) {
			return vault.SecretsByIDs(ctx, o.IDs...)
		})

	case len(o.Name) > 0 && len(o.Labels) > 0:
		return sortAndMarkSecrets(ctx, vault, func() (map[int]store.SecretWithLabels, error) {
			return vault.SecretsByLabelsAndName(ctx, o.Name, o.Labels...)
		})

	case len(o.Name) > 0:
		return sortAndUnmarkSecrets(func() (map[int]store.SecretWithLabels, error) {
			return vault.SecretsByName(ctx, o.Name)
		})

	case len(o.Labels) > 0:
		return sortAndMarkSecrets(ctx, vault, func() (map[int]store.SecretWithLabels, error) {
			return vault.SecretsByLabels(ctx, o.Labels...)
		})

	default:
		return sortAndUnmarkSecrets(func() (map[int]store.SecretWithLabels, error) {
			return vault.SecretsWithLabels(ctx)
		})
	}
}

func (o *SearchableOptions) Validate() error {
	c := 0

	if len(o.IDs) > 0 {
		c++
	}

	if len(o.Labels) > 0 {
		c++
	}

	if len(o.Name) > 0 {
		c++
	}

	if c == 0 {
		return ErrNoSearchArgs
	}

	return nil
}

type secretWithLabels struct {
	id     int
	name   string
	labels []string
}

type secretWithMarkedLabels struct {
	id     int
	name   string
	labels []markedLabel
}

// markedLabel represents a label and whether it matched a filter.
type markedLabel struct {
	value   string
	matched bool
}

type retrieveSecretsFunc func() (map[int]store.SecretWithLabels, error)

// sortAndUnmarkSecrets returns secrets with all their labels, ordered by id value.
func sortAndUnmarkSecrets(retrieveSecretsFunc retrieveSecretsFunc) ([]secretWithMarkedLabels, error) {
	secrets, err := retrieveSecretsFunc()
	if err != nil {
		return nil, err
	}

	sortedByID := secretsMapToSlice(secrets)
	slices.SortFunc(sortedByID, func(a, b secretWithLabels) int {
		return b.id - a.id
	})

	return secretsWithUnmarkedLabels(sortedByID), nil
}

// sortAndMarkSecrets returns secrets with all their labels, ordered in
// descending order by the number of labels initially matched by retrieveMatchingFunc.
// Labels matched by retrieveMatchingFunc are marked as matched via [markedLabel].
//
// retrieveMatchingFunc typically returns secrets containing only the labels
// that match the applied filter.
func sortAndMarkSecrets(ctx context.Context, vault *vlt.Vault, retrieveMatchingFunc retrieveSecretsFunc) ([]secretWithMarkedLabels, error) {
	matchingSecrets, err := retrieveMatchingFunc()
	if err != nil {
		return nil, err
	}

	if len(matchingSecrets) == 0 {
		return nil, nil
	}

	// Sort in descending order by label count
	sortedByLabelsCount := secretsMapToSlice(matchingSecrets)
	slices.SortFunc(sortedByLabelsCount, func(a, b secretWithLabels) int {
		return len(b.labels) - len(a.labels)
	})

	sortedIDs := make([]int, len(sortedByLabelsCount))
	for i, secret := range sortedByLabelsCount {
		sortedIDs[i] = secret.id
	}

	secrets, err := vault.SecretsByIDs(ctx, sortedIDs...)
	if err != nil {
		return nil, err
	}

	sortedSecrets := make([]secretWithMarkedLabels, len(secrets))
	for i, id := range sortedIDs {
		sortedSecrets[i] = secretWithMarkedLabels{
			id:     id,
			name:   secrets[id].Name,
			labels: markMatchingLabels(secrets[id].Labels, matchingSecrets[id].Labels),
		}
	}

	return sortedSecrets, nil
}

func secretsWithUnmarkedLabels(ordered []secretWithLabels) []secretWithMarkedLabels {
	marked := make([]secretWithMarkedLabels, len(ordered))
	for i, o := range ordered {
		marked[i] = secretWithMarkedLabels{
			id:     o.id,
			name:   o.name,
			labels: markMatchingLabels(o.labels, nil),
		}
	}

	return marked
}

func secretsMapToSlice(m map[int]store.SecretWithLabels) []secretWithLabels {
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

func markMatchingLabels(labels []string, matchingLabels []string) []markedLabel {
	marked := make([]markedLabel, len(labels))
	for i, l := range labels {
		marked[i] = markedLabel{
			value:   l,
			matched: slices.Contains(matchingLabels, l),
		}
	}

	return marked
}

func extractIDs(secrets []secretWithMarkedLabels) []int {
	ids := make([]int, len(secrets))
	for i, s := range secrets {
		ids[i] = s.id
	}

	return ids
}

// ANSI color codes for formatting.
const (
	greenBold = "\033[1;32m"
	reset     = "\033[0m"
)

// highlight adds bold green formatting to a string.
func highlight(s string) string {
	return greenBold + s + reset
}

func highlightMarked(labels []markedLabel) []string {
	hl := make([]string, len(labels))
	for i, l := range labels {
		if l.matched {
			hl[i] = highlight(l.value)
		} else {
			hl[i] = l.value
		}
	}

	return hl
}

func printTable(w io.Writer, markedLabeledSecrets []secretWithMarkedLabels) {
	tw := tabwriter.NewWriter(w, 0, 0, 5, ' ', 0)
	defer func() { _ = tw.Flush() }()

	fmt.Fprintln(tw, "ID\tNAME\tLABELS")

	for _, marked := range markedLabeledSecrets {
		fmt.Fprintf(tw, "%d\t%s", marked.id, marked.name)

		highlightedLabels := highlightMarked(marked.labels)

		if len(highlightedLabels) == 0 {
			fmt.Fprintf(tw, "\t\t\n")
			continue
		}

		for i, label := range highlightedLabels {
			if i == 0 {
				fmt.Fprintf(tw, "\t%s\n", label)
				continue
			}

			fmt.Fprintf(tw, "\t\t%s\n", label)
		}
	}

	fmt.Fprintln(tw) // add padding
}
