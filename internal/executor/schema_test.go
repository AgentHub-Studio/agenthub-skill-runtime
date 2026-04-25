package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenantSchemaQuotesTenantID(t *testing.T) {
	assert.Equal(t, `"ah_test"`, TenantSchema("test"))
	assert.Equal(t, `"ah_test"";drop schema public;--"`, TenantSchema(`test";drop schema public;--`))
}
