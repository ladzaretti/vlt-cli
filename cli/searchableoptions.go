package cli

import (
	"context"
	"slices"

	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"
	"github.com/ladzaretti/vlt-cli/vlt/store"
)

type SearchableOptions struct {
	*genericclioptions.SearchOptions
}

// search queries the vault for secrets based on the fields
// set in [genericclioptions.SearchOptions].
//
// For any matched secret, it returns all labels associated with it,
// regardless of the filter options used.
// The resulting slice of pairs is ordered by label count in descending order.
func (o *SearchableOptions) search(ctx context.Context, vault *vlt.Vault) ([]markedLabeledSecret, error) {
	switch {
	case len(o.IDs) > 0:
		return markSecrets(sortSecrets(vault.SecretsByIDs(ctx, o.IDs...)))

	case len(o.Name) > 0 && len(o.Labels) > 0:
		return sortFullSecrets(ctx, vault, func() (map[int]store.LabeledSecret, error) {
			return vault.SecretsByLabelsAndName(ctx, o.Name, o.Labels...)
		})

	case len(o.Name) > 0:
		return sortFullSecrets(ctx, vault, func() (map[int]store.LabeledSecret, error) {
			return vault.SecretsByName(ctx, o.Name)
		})

	case len(o.Labels) > 0:
		return sortFullSecrets(ctx, vault, func() (map[int]store.LabeledSecret, error) {
			return vault.SecretsByLabels(ctx, o.Labels...)
		})

	default:
		return markSecrets(sortSecrets(vault.SecretsWithLabels(ctx)))
	}
}

type sortedLabeledSecret struct {
	id     int
	name   string
	labels []string
}

type markedLabeledSecret struct {
	id     int
	name   string
	labels []markedLabel
}

// markedLabel marks whether a label matched the filter.
type markedLabel struct {
	value   string
	matched bool
}

type matchSecretsFunc func() (map[int]store.LabeledSecret, error)

// sortFullSecrets returns full secrets (i.e., with all labels), ordered in
// descending order by the matched label count returned by matchSecrets.
//
// matchSecrets typically returns secrets with only the labels
// that match the applied filter.
func sortFullSecrets(ctx context.Context, vault *vlt.Vault, matchSecrets matchSecretsFunc) ([]markedLabeledSecret, error) {
	matched, err := matchSecrets()
	if err != nil {
		return nil, err
	}

	if len(matched) == 0 {
		return nil, nil
	}

	matchedSorted := sortByLabelsCount(matched)

	sortedIDs := make([]int, len(matchedSorted))
	for i, secret := range matchedSorted {
		sortedIDs[i] = secret.id
	}

	fullSecrets, err := vault.SecretsByIDs(ctx, sortedIDs...)
	if err != nil {
		return nil, err
	}

	fullSorted := make([]markedLabeledSecret, len(fullSecrets))
	for i, id := range sortedIDs {
		fullSorted[i] = markedLabeledSecret{
			id:     id,
			name:   fullSecrets[id].Name,
			labels: markMatchedLabels(fullSecrets[id].Labels, matched[id].Labels),
		}
	}

	return fullSorted, nil
}

func sortSecrets(m map[int]store.LabeledSecret, err error) ([]sortedLabeledSecret, error) {
	if err != nil {
		return nil, err
	}

	return sortByLabelsCount(m), nil
}

func markSecrets(ordered []sortedLabeledSecret, err error) ([]markedLabeledSecret, error) {
	if err != nil {
		return nil, err
	}

	marked := make([]markedLabeledSecret, len(ordered))
	for i, o := range ordered {
		marked[i] = markedLabeledSecret{
			id:     o.id,
			name:   o.name,
			labels: markMatchedLabels(o.labels, nil),
		}
	}

	return marked, nil
}

// sortByLabelsCount takes a map of labeled secrets and
// returns a slice sorted by descending label count.
func sortByLabelsCount(m map[int]store.LabeledSecret) []sortedLabeledSecret {
	sorted := make([]sortedLabeledSecret, 0, len(m))
	for id, labeled := range m {
		l := sortedLabeledSecret{
			id:     id,
			name:   labeled.Name,
			labels: labeled.Labels,
		}
		sorted = append(sorted, l)
	}

	// Sort in descending order by label count
	slices.SortFunc(sorted, func(a, b sortedLabeledSecret) int {
		return len(b.labels) - len(a.labels)
	})

	return sorted
}

func markMatchedLabels(labels []string, matchingLabels []string) []markedLabel {
	marked := make([]markedLabel, len(labels))
	for i, l := range labels {
		marked[i] = markedLabel{
			value:   l,
			matched: slices.Contains(matchingLabels, l),
		}
	}

	return marked
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
