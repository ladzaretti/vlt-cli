package util

import "strings"

func ParseCommaSeparated(raw string) []string {
	res := make([]string, 0, 8)

	split := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' })
	for _, s := range split {
		if l := strings.TrimSpace(s); len(l) > 0 {
			res = append(res, l)
		}
	}

	return res
}

func ToAnySlice[T any](ts []T) []any {
	args := make([]any, len(ts))

	for i, t := range ts {
		args[i] = t
	}

	return args
}
