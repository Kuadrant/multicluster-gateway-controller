//go:build unit

package env

import (
	"os"
	"testing"
)

// These tests cannot be run in parallel and should be updated to use testing.SetEnv if/when we update to go 1.17+ https://pkg.go.dev/testing#B.Setenv

func TestGetEnvBool(t *testing.T) {
	setupTestEnv(t)
	defer teardownTestEnv(t)
	type args struct {
		key      string
		fallback bool
	}
	testCases := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "returns fallback",
			args: args{
				key:      "MGC_TST_NO_ENVAR",
				fallback: false,
			},
			want: false,
		},
		{
			name: "returns env var value",
			args: args{
				key:      "MGC_TST_FALSE_BOOL",
				fallback: true,
			},
			want: false,
		},
		{
			name: "returns fallback for non bool env var value",
			args: args{
				key:      "MGC_TST_NOT_BOOL",
				fallback: false,
			},
			want: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := GetEnvBool(testCase.args.key, testCase.args.fallback); got != testCase.want {
				t.Errorf("GetEnvBool() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestGetEnvString(t *testing.T) {
	setupTestEnv(t)
	defer teardownTestEnv(t)

	type args struct {
		key      string
		fallback string
	}
	testCases := []struct {
		name string
		args args
		want string
	}{
		{
			name: "returns fallback",
			args: args{
				key:      "MGC_TST_NO_ENVAR",
				fallback: "bar",
			},
			want: "bar",
		},
		{
			name: "returns env var value",
			args: args{
				key:      "MGC_TST_FOO_STR",
				fallback: "bar",
			},
			want: "foo",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := GetEnvString(testCase.args.key, testCase.args.fallback); got != testCase.want {
				t.Errorf("GetEnvString() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func setupTestEnv(t *testing.T) {
	_ = os.Setenv("MGC_TST_FALSE_BOOL", "false")
	_ = os.Setenv("MGC_TST_NOT_BOOL", "notabool")
	_ = os.Setenv("MGC_TST_FOO_STR", "foo")
}

func teardownTestEnv(t *testing.T) {
	_ = os.Unsetenv("MGC_TST_FALSE_BOOL")
	_ = os.Unsetenv("MGC_TST_NOT_BOOL")
	_ = os.Unsetenv("MGC_TST_FOO_STR")
}
