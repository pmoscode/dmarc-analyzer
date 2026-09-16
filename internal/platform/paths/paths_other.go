//go:build !darwin

package paths

import "os"

// userLogDir liefert unter Linux und Windows dasselbe Verzeichnis wie
// os.UserConfigDir() — dort existieren keine dedizierten Log-Pfade als
// Betriebssystemkonvention wie unter macOS.
func userLogDir() (string, error) {
	return os.UserConfigDir()
}
