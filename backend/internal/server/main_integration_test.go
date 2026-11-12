//go:build integration

package server_test

import (
	"testing"

	"github.com/freewhen/backend/internal/testdb"
)

// TestMain stops the shared embedded Postgres when the binary exits.
func TestMain(m *testing.M) { testdb.Main(m) }
