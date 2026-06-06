package storage

import (
	"sort"
	"strings"
)

// usageAgg accumulates a recursive listing into totals + an immediate-child
// breakdown, shared by the real client and the Fake so both rank identically
// (005 FR-008/FR-009). The immediate child of a key under prefix is its first path
// segment after prefix: a sub-prefix (trailing "/") when more "/" follow, else the
// direct object.
type usageAgg struct {
	bucket     string
	prefix     string
	totalSize  int64
	totalCount int
	children   map[string]*UsageChild
}

func newUsageAgg(bucket, prefix string) *usageAgg {
	return &usageAgg{bucket: bucket, prefix: prefix, children: map[string]*UsageChild{}}
}

// add folds one object (full key + size) into the aggregate. A key equal to the
// prefix itself (the folder placeholder) contributes nothing.
func (a *usageAgg) add(key string, size int64) {
	if !strings.HasPrefix(key, a.prefix) {
		return
	}
	rest := key[len(a.prefix):]
	if rest == "" {
		return // the prefix's own placeholder object — not a child
	}
	a.totalSize += size
	a.totalCount++

	name, isDir := rest, false
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		name, isDir = rest[:idx+1], true // immediate sub-prefix incl trailing "/"
	}
	ch, ok := a.children[name]
	if !ok {
		ch = &UsageChild{Name: name, IsDir: isDir}
		a.children[name] = ch
	}
	ch.Size += size
	ch.Count++
}

// report materializes the aggregate into a UsageReport, ranking children by size
// descending (ties broken by name ascending). complete marks a finished scan.
func (a *usageAgg) report(complete bool) UsageReport {
	children := make([]UsageChild, 0, len(a.children))
	for _, ch := range a.children {
		children = append(children, *ch)
	}
	sort.Slice(children, func(i, j int) bool {
		if children[i].Size != children[j].Size {
			return children[i].Size > children[j].Size
		}
		return children[i].Name < children[j].Name
	})
	return UsageReport{
		Bucket:     a.bucket,
		Prefix:     a.prefix,
		TotalSize:  a.totalSize,
		TotalCount: a.totalCount,
		Children:   children,
		Complete:   complete,
	}
}
