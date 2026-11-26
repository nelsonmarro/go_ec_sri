//go:build sriintegration

package soap

import (
	"context"
	"testing"
	"time"

	"github.com/nelsonmarro/go_ec_sri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Smoke test against the real SRI test environment. It never runs by default:
// go test -tags sriintegration ./soap
func TestSRIIntegration_AuthorizeUnknownKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a, err := New().Authorize(ctx, sri.EnvironmentTest, "0101202601179000000200110010010000000011234567814")
	if err != nil {
		t.Skipf("SRI unavailable: %v", err)
	}
	require.NotNil(t, a)
	assert.Equal(t, sri.StatusNone, a.Status)
}
