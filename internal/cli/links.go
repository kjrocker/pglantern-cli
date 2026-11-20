package cli

import (
	"strconv"
	"strings"
)

// Canonical link shapes for the entities we mirror. These replicate the
// server's HortonWeb.PgLinks (lib/horton_web/pg_links.ex) so `open` can build a
// URL client-side without a round-trip — keep the two in sync when either
// changes.
const archiveBase = "https://postgr.es"

// encodeUnreserved percent-encodes to match the server's
// URI.encode(s, &URI.char_unreserved?/1): keep the RFC 3986 unreserved set
// (A-Za-z0-9 and -_.~) verbatim, percent-encode every other byte. Byte-wise, so
// multibyte UTF-8 is encoded per byte exactly as URI.encode does. This is
// stricter than the url.PathEscape used for API paths (which leaves `@`, `=`,
// `+`, and friends unescaped).
func encodeUnreserved(s string) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperhex[c>>4])
		b.WriteByte(upperhex[c&0x0f])
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-', c == '_', c == '.', c == '~':
		return true
	}
	return false
}

// id passed to the message builders is already normalizeMessageID'd
// (bracket-stripped), matching PgLinks.message_url/1 which routes on the
// bracket-free id.
func messageArchiveURL(id string) string { return archiveBase + "/m/" + encodeUnreserved(id) }
func messageSiteURL(host, id string) string {
	return trimHost(host) + "/messages/" + encodeUnreserved(id)
}
func commitArchiveURL(sha string) string    { return archiveBase + "/c/" + sha }
func commitSiteURL(host, sha string) string { return trimHost(host) + "/commits/" + sha }

const cfAppBase = "https://commitfest.postgresql.org"

// Commitfest and cf-patch ids are the upstream app's numeric ids.
func commitfestArchiveURL(id string) string { return cfAppBase + "/" + id + "/" }
func commitfestSiteURL(host, id string) string {
	return trimHost(host) + "/commitfests/" + id
}
func cfPatchArchiveURL(id string) string { return cfAppBase + "/patch/" + id + "/" }
func cfPatchSiteURL(host, id string) string {
	return trimHost(host) + "/cf-patches/" + id
}

// isCfID reports whether s is a positive decimal id, the only shape the
// commitfest pages route on — anything else would open a 404.
func isCfID(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0 && strconv.Itoa(n) == s
}

// trimHost drops a trailing slash so we never emit `//messages`.
func trimHost(host string) string { return strings.TrimRight(host, "/") }

// isFullSHA reports whether s is a full 40-hex sha, which `commits open` can
// turn into a URL client-side; anything shorter is a prefix that needs
// server-side resolution.
func isFullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
