package web

// shortCommitLen ist die Länge, auf die der Commit-Hash in der Oberfläche
// gekürzt wird — dieselbe Länge wie "git log --oneline" üblicherweise.
const shortCommitLen = 7

// BuildInfo beschreibt den laufenden Build (Version und Git-Commit) für
// die Anzeige unter dem Schriftzug in der Kopfzeile (layout.html). Ermittelt wird beides in der
// Composition Root (cmd/dmarc-analyzer/version.go), das Web-Paket zeigt es
// nur an.
type BuildInfo struct {
	// Version ist z. B. ein Tag ("v1.2.0") oder "dev".
	Version string
	// Commit ist der vollständige Git-Commit-Hash, leer, wenn unbekannt.
	Commit string
}

// ShortCommit liefert den auf shortCommitLen gekürzten Commit-Hash. Ein
// Suffix wie "-dirty" (siehe cmd/dmarc-analyzer/version.go) bleibt dabei
// erhalten, damit ein Build aus einem veränderten Arbeitsverzeichnis in der
// Oberfläche erkennbar bleibt.
func (b BuildInfo) ShortCommit() string {
	hash, suffix := b.Commit, ""
	for i, r := range b.Commit {
		if r == '-' {
			hash, suffix = b.Commit[:i], b.Commit[i:]
			break
		}
	}
	if len(hash) > shortCommitLen {
		hash = hash[:shortCommitLen]
	}
	return hash + suffix
}
