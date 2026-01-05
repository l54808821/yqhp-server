package master

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute_Help(t *testing.T) {
	// Test help command
	err := Execute([]string{"help"})
	assert.NoError(t, err)

	err = Execute([]string{"-h"})
	assert.NoError(t, err)

	err = Execute([]string{"--help"})
	assert.NoError(t, err)
}

func TestExecute_NoArgs(t *testing.T) {
	// Test with no arguments - should print usage
	err := Execute([]string{})
	assert.NoError(t, err)
}

func TestExecute_UnknownSubcommand(t *testing.T) {
	err := Execute([]string{"unknown"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown master subcommand")
}

func TestExecuteStatus_Default(t *testing.T) {
	// Test status command with default address
	err := executeStatus([]string{})
	assert.NoError(t, err)
}

func TestExecuteStatus_CustomAddress(t *testing.T) {
	// Test status command with custom address
	err := executeStatus([]string{"-address", "http://custom:8080"})
	assert.NoError(t, err)
}
