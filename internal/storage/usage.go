package storage

import (
	"sort"
	"strings"
	"time"
)

// usageAgg accumulates a recursive listing into totals + an immediate-child
// breakdown + age/size/class distributions, shared by the real client and the Fake
// so both rank and bucket identically (005 FR-008/FR-009, 017 US4). The immediate
// child of a key under prefix is its first path segment after prefix: a sub-prefix
// (trailing "/") when more "/" follow, else the direct object.
type usageAgg struct {
	bucket     string
	prefix     string
	scanStart  time.Time
	totalSize  int64
	totalCount int
	children   map[string]*UsageChild
	ageDist    [6]DistBucket
	sizeDist   [6]DistBucket
	classDist  map[string]DistBucket
}

func newUsageAgg(bucket, prefix string, scanStart time.Time) *usageAgg {
	return &usageAgg{
		bucket:    bucket,
		prefix:    prefix,
		scanStart: scanStart,
		children:  map[string]*UsageChild{},
		classDist: map[string]DistBucket{},
	}
}

// Size-histogram boundaries, index-aligned with SizeBucketLabels: a size belongs to
// the first bucket whose upper edge exceeds it; the last bucket is unbounded.
var sizeBucketEdges = [5]int64{
	128 << 10, // 128 KiB
	1 << 20,   // 1 MiB
	16 << 20,  // 16 MiB
	128 << 20, // 128 MiB
	1 << 30,   // 1 GiB
}

// Age-histogram boundaries, index-aligned with AgeBucketLabels.
var ageBucketEdges = [5]time.Duration{
	24 * time.Hour,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
	90 * 24 * time.Hour,
	365 * 24 * time.Hour,
}

func sizeBucketIdx(size int64) int {
	for i, edge := range sizeBucketEdges {
		if size < edge {
			return i
		}
	}
	return len(sizeBucketEdges)
}

func ageBucketIdx(age time.Duration) int {
	for i, edge := range ageBucketEdges {
		if age < edge {
			return i
		}
	}
	return len(ageBucketEdges)
}

// add folds one object (full key + size + last-modified + storage class) into the
// aggregate. A key equal to the prefix itself (the folder placeholder) contributes
// nothing. Distribution accumulation is O(1) per object — the same pass that already
// touches every entry (017 FR-020).
func (a *usageAgg) add(key string, size int64, lastModified time.Time, storageClass string) {
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

	ai := ageBucketIdx(a.scanStart.Sub(lastModified))
	a.ageDist[ai].Count++
	a.ageDist[ai].Size += size

	si := sizeBucketIdx(size)
	a.sizeDist[si].Count++
	a.sizeDist[si].Size += size

	class := storageClass
	if class == "" {
		class = "STANDARD"
	}
	cb := a.classDist[class]
	cb.Count++
	cb.Size += size
	a.classDist[class] = cb
}

// report materializes the aggregate into a UsageReport, ranking children by size
// descending (ties broken by name ascending). complete marks a finished scan; bounded
// marks a scan stopped by the maxObjects cap (never both true — data-model §2).
func (a *usageAgg) report(complete, bounded bool) UsageReport {
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
		Bounded:    bounded,
		ScanStart:  a.scanStart,
		AgeDist:    a.ageDist,
		SizeDist:   a.sizeDist,
		ClassDist:  a.classDist,
	}
}
