package logging_test

import (
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
