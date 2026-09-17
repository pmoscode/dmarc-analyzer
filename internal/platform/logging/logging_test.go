package logging_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/logging"
)

func TestNew_ReturnsNonNilLogger(t *testing.T) {
	t.Parallel()

	logger := logging.New()

	require.NotNil(t, logger)
}

func TestNew_SetsSlogDefault(t *testing.T) {
	logger := logging.New(logging.WithJSON(true), logging.WithLevel(slog.LevelDebug))

	require.Same(t, logger, slog.Default())
}

func TestNew_WithWriter_WritesToGivenWriterInsteadOfStderr(t *testing.T) {
	var buf bytes.Buffer

	logger := logging.New(logging.WithWriter(&buf))
	logger.Info("testnachricht")

	require.Contains(t, buf.String(), "testnachricht")
}
