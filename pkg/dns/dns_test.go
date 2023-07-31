//go:build unit

package dns

import (
	"errors"
	"testing"
)

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectedError string
	}{
		{
			name:          "error message with request id",
			err:           errors.New("An error has occurred, request id: 12345abcd"),
			expectedError: "An error has occurred, ",
		},
		{
			name:          "error message without request id",
			err:           errors.New("An error has occurred"),
			expectedError: "An error has occurred",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeError(tt.err)
			if got.Error() != tt.expectedError {
				t.Errorf("expected '%v' got '%v'", tt.expectedError, got)
			}
		})
	}
}
