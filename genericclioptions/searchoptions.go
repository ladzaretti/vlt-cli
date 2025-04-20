package genericclioptions

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
	ID:     "filter by secret ID (comma-separated or repeated)",
	NAME:   "filter by secret name",
	LABELS: "filter by secret label (comma-separated or repeated)",
}

var _ BaseOptions = &SearchOptions{}

func (*SearchOptions) Usage(field Usage) string {
	if u, ok := usage[field]; ok {
		return u
	}

	return "unknown usage"
}

func (*SearchOptions) Complete() error {
	return nil
}

func (*SearchOptions) Validate() error {
	return nil
}
