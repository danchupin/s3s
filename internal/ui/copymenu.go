package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/share"
	"github.com/danchupin/s3s/internal/storage"
)

// Copy & share menu (017 US3): ONE `Y` key (and the `:copy` command — same dispatcher)
// opens a focus-aware overlay of copyable artifacts. Copy = the best-effort OSC52 path
// reveal already uses; `i` on an item shows the value in the reveal popup for manual
// copy (the no-clipboard fallback, FR-019). The presigned link is a bearer secret —
// shown and copied, NEVER logged (FR-016).

type copyKind int

const (
	copyKindURI copyKind = iota
	copyKindURL
	copyKindCmd
	copyKindPresign
	copyKindPresignCurl
	copyKindField
	copyKindExportCSV
	copyKindExportJSON
)

type copyItem struct {
	label string
	kind  copyKind
}

// copyMenuState is the open overlay (nil = closed).
type copyMenuState struct {
	bucket, key string // target; key carries a trailing "/" for a prefix, "" for a bucket
	isObject    bool
	items       []copyItem
	sel         int
	ttlPick     bool // the TTL sub-picker is open (for a presign item)
	ttlKind     copyKind
	ttlSel      int // index into storage.PresignTTLs; default 1 (1h)
}

// copyMenuTarget resolves what the menu acts on for the current focus.
func (m App) copyMenuTarget() (bucket, key string, isObject, ok bool) {
	if m.mode == modeObject && m.meta != nil {
		return m.bucket, m.meta.Key, true, true
	}
	if m.inLevel() {
		if e := m.selected(); e != nil {
			return m.bucket, e.full, !e.isDir, true
		}
		if m.bucket != "" {
			return m.bucket, m.prefix, false, true
		}
	}
	if m.mode == modeBuckets {
		if b := m.highlightedBucketName(); b != "" {
			return b, "", false, true
		}
	}
	return "", "", false, false
}

// openCopyMenu builds the focus-aware item list and opens the overlay (FR-014).
func (m App) openCopyMenu() (tea.Model, tea.Cmd) {
	bucket, key, isObject, ok := m.copyMenuTarget()
	if !ok {
		return m, nil
	}
	items := []copyItem{{label: "S3 URI", kind: copyKindURI}}
	if isObject {
		items = append(items,
			copyItem{label: "HTTPS URL", kind: copyKindURL},
			copyItem{label: "download command", kind: copyKindCmd},
			copyItem{label: "presigned link…", kind: copyKindPresign},
			copyItem{label: "presigned curl…", kind: copyKindPresignCurl},
			copyItem{label: "copy a field…", kind: copyKindField},
		)
	}
	// A visible usage report for the focused bucket/prefix target adds its exports.
	if b, p, tok := m.fullScanTarget(); tok && !isObject {
		if _, hit := m.usageResults.Get(m.usageKey(b, p)); hit {
			items = append(items,
				copyItem{label: "export CSV", kind: copyKindExportCSV},
				copyItem{label: "export JSON", kind: copyKindExportJSON},
			)
		}
	}
	m.copyMenu = &copyMenuState{bucket: bucket, key: key, isObject: isObject, items: items, ttlSel: 1}
	return m, nil
}

// copyArtifact builds the immediate (non-presign) artifact value for an item.
func (m App) copyArtifact(cm *copyMenuState, kind copyKind) string {
	switch kind {
	case copyKindURI:
		return share.S3URI(cm.bucket, cm.key)
	case copyKindURL:
		return share.HTTPURL(m.info.Endpoint, m.info.PathStyle, cm.bucket, cm.key)
	case copyKindCmd:
		return share.CLISnippet(m.info.Endpoint, cm.bucket, cm.key)
	}
	return ""
}

// onCopyMenuKey owns input while the menu (or its TTL sub-picker) is open.
func (m App) onCopyMenuKey(key string) (tea.Model, tea.Cmd) {
	cm := m.copyMenu
	if cm.ttlPick {
		switch {
		case matches(key, m.keys.Up):
			if cm.ttlSel > 0 {
				cm.ttlSel--
			}
		case matches(key, m.keys.Down):
			if cm.ttlSel < len(storage.PresignTTLs)-1 {
				cm.ttlSel++
			}
		case matches(key, m.keys.Enter):
			ttl := storage.PresignTTLs[cm.ttlSel]
			bucket, objKey, curl := cm.bucket, cm.key, cm.ttlKind == copyKindPresignCurl
			m.copyMenu = nil
			return m, m.presignCmd(bucket, objKey, ttl, curl)
		case matches(key, m.keys.Back):
			cm.ttlPick = false
		}
		return m, nil
	}

	switch {
	case matches(key, m.keys.Up):
		if cm.sel > 0 {
			cm.sel--
		}
	case matches(key, m.keys.Down):
		if cm.sel < len(cm.items)-1 {
			cm.sel++
		}
	case matches(key, m.keys.Reveal):
		// Show-value fallback: the artifact in the reveal popup for manual copy (FR-019).
		if v := m.copyArtifact(cm, cm.items[cm.sel].kind); v != "" {
			m.copyMenu = nil
			m.reveal = &revealState{kind: revealPath, value: v}
		}
	case matches(key, m.keys.Enter):
		it := cm.items[cm.sel]
		switch it.kind {
		case copyKindURI, copyKindURL, copyKindCmd:
			v := m.copyArtifact(cm, it.kind)
			m.copyMenu = nil
			m.notice = "copied " + it.label + " — " + truncate(v, 48)
			return m, tea.SetClipboard(v)
		case copyKindPresign, copyKindPresignCurl:
			cm.ttlPick = true
			cm.ttlKind = it.kind
		case copyKindField:
			m.copyMenu = nil
			return m.startFieldCopy()
		case copyKindExportCSV, copyKindExportJSON:
			format := "csv"
			if it.kind == copyKindExportJSON {
				format = "json"
			}
			b, p, ok := m.fullScanTarget()
			if !ok {
				m.copyMenu = nil
				return m, nil
			}
			rep, hit := m.usageResults.Get(m.usageKey(b, p))
			m.copyMenu = nil
			if !hit {
				return m, nil
			}
			return m, m.exportReportCmd(*rep, format)
		}
	case matches(key, m.keys.Back):
		m.copyMenu = nil
	}
	return m, nil
}

// presignCmd mints the link off the event loop (the credential chain may block —
// constitution II) and reports back via presignDoneMsg. The URL is never logged here
// or in storage (FR-016).
func (m App) presignCmd(bucket, key string, ttl time.Duration, curl bool) tea.Cmd {
	st := m.activeStore()
	return func() tea.Msg {
		u, warn, err := st.PresignGet(context.Background(), bucket, key, ttl)
		return presignDoneMsg{key: key, url: u, warn: warn, err: err, ttl: ttl, curl: curl}
	}
}

// onPresignDone copies the link (or its curl form) and confirms — appending the
// credential-expiry warning when the backend raised one (FR-015/FR-017).
func (m App) onPresignDone(msg presignDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	v := msg.url
	what := "presigned link"
	if msg.curl {
		v = share.CurlSnippet(msg.url, msg.key)
		what = "presigned curl"
	}
	m.notice = fmt.Sprintf("copied %s (%s) — %s", what, ttlLabel(msg.ttl), truncate(msg.key, 40))
	if msg.warn != "" {
		m.notice += "  ⚠ " + msg.warn
	}
	return m, tea.SetClipboard(v)
}

// exportReportCmd writes the report into DownloadDir (temp+rename; the temp is removed
// on failure so no partial file masquerades as a report) and logs only metadata
// (FR-018). The filename is stamped with the injectable clock for deterministic tests.
func (m App) exportReportCmd(rep storage.UsageReport, format string) tea.Cmd {
	dir := m.info.DownloadDir
	if dir == "" {
		dir = "."
	}
	name := share.ReportFileName(rep.Bucket, rep.Prefix, m.now(), format)
	return func() tea.Msg {
		var data []byte
		if format == "json" {
			data = share.ExportJSON(&rep)
		} else {
			data = share.ExportCSV(&rep)
		}
		path := filepath.Join(dir, name)
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			_ = os.Remove(tmp)
			return exportDoneMsg{err: err}
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return exportDoneMsg{err: err}
		}
		slog.Info("export", "path", path, "format", format, "rows", strings.Count(string(data), "\n"))
		return exportDoneMsg{path: path}
	}
}

// onExportDone surfaces the outcome (FR-018: failures are reported, never silent).
func (m App) onExportDone(msg exportDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.notice = "exported — " + msg.path
	return m, nil
}

// ttlLabel renders a preset TTL compactly (15m/1h/24h/7d).
func ttlLabel(ttl time.Duration) string {
	switch ttl {
	case 15 * time.Minute:
		return "15m"
	case time.Hour:
		return "1h"
	case 24 * time.Hour:
		return "24h"
	case 7 * 24 * time.Hour:
		return "7d"
	}
	return ttl.String()
}

// copyMenuView renders the overlay (or its TTL sub-picker) centered over the body,
// reusing the popup surface + shared key-glyph vocabulary (constitution VII).
func (m App) copyMenuView(w, h int) string {
	cm := m.copyMenu
	var body string
	if cm.ttlPick {
		for i, ttl := range storage.PresignTTLs {
			line := ttlLabel(ttl)
			if i == cm.ttlSel {
				body += selRowStyle.Render("▸ "+line) + "\n"
			} else {
				body += objCellStyle.Render("  "+line) + "\n"
			}
		}
		inner := titleStyle.Render("link validity") + "\n\n" +
			strings.TrimRight(body, "\n") + "\n\n" +
			dimCellStyle.Render(keyHint(m.keys.Enter, "sign & copy")+sepDot+keyHint(m.keys.Back, "back"))
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popupBoxStyle.Render(inner))
	}
	for i, it := range cm.items {
		if i == cm.sel {
			body += selRowStyle.Render("▸ "+it.label) + "\n"
		} else {
			body += objCellStyle.Render("  "+it.label) + "\n"
		}
	}
	target := share.S3URI(cm.bucket, cm.key)
	inner := titleStyle.Render("copy / share") + "\n" +
		dimCellStyle.Render(truncate(target, max(10, min(w-12, 64)))) + "\n\n" +
		strings.TrimRight(body, "\n") + "\n\n" +
		dimCellStyle.Render(keyHint(m.keys.Enter, "copy")+sepDot+keyHint(m.keys.Reveal, "show value")+sepDot+keyHint(m.keys.Back, "cancel"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popupBoxStyle.Render(inner))
}
