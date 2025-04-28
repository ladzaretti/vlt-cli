package genericclioptions

import "errors"

var ErrNoSearchParams = errors.New("no search criteria provided; specify at least one of --id, --label, or --name")

// SearchOptions defines common filtering options for CLI commands that
// support filtering secrets.
type SearchOptions struct {
	IDs    []int
	Name   string
	Labels []string
}

type Usage int

const (
	_ Usage = iota
	ID
	NAME
	LABELS
)

var usage = map[Usage]string{
	ID:     "search by ID",
	NAME:   "search by name",
	LABELS: "search by label (comma-separated or repeated)",
}

var _ BaseOptions = &SearchOptions{}

func (*SearchOptions) Usage(field Usage) string {
	if u, ok := usage[field]; ok {
		return u
	}

	return "unknown usage"
}

func (s *SearchOptions) IsUnset() bool {
	return len(s.Name) == 0 && len(s.Labels) == 0 && len(s.IDs) == 0
}

func (*SearchOptions) Complete() error {
	return nil
}

func (*SearchOptions) Validate() error {
	return nil
}
