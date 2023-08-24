package log_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func addRule(t *testing.T, logger log.Logger, entity log.SemEntity, rule string) {
	seml, ok := logger.(log.SemLogger)
	require.True(t, ok, "Bad test setup. Expected a SEM logger but it isn't")
	seml.AddRule(log.SemTransaction, []byte(rule))
}

// helper to set a SEM logger in active state (add rule & enter matching that rule)
func goActive(t *testing.T, logger log.Logger) {
	seml, ok := logger.(log.SemLogger)
	require.True(t, ok, "Bad test setup. Expected a SEM logger but it isn't")
	ruleVal := "myRuleVal"
	addRule(t, logger, log.SemTransaction, ruleVal)
	log.SemEntry(logger, log.SemTransaction, []byte(ruleVal))
	status, err := seml.Status()
	require.NoError(t, err)
	require.True(t, status.Active)

}

// Testing SEM logger wrapping TMLLogger
func TestSemWithTMLogger(t *testing.T) {
	testCases := []struct {
		name          string
		active        bool
		loggerBackend func(io.Writer) log.Logger
		want          string
	}{
		{
			"RuleEnabled with NewTMJSONLoggerNoTS",
			true,
			func(w io.Writer) log.Logger { return log.NewTMJSONLoggerNoTS(w) },
			strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"RuleDisabled NewTMJSONLoggerNoTS",
			false,
			func(w io.Writer) log.Logger { return log.NewTMJSONLoggerNoTS(w) },
			strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"RuleEnabled with Filtered AllowAll",
			true,
			func(w io.Writer) log.Logger { return log.NewFilter(log.NewTMJSONLoggerNoTS(w), log.AllowAll()) },
			strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"RuleDisabled with Filtered AllowAll",
			false,
			func(w io.Writer) log.Logger { return log.NewFilter(log.NewTMJSONLoggerNoTS(w), log.AllowAll()) },
			strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"RuleEnabled with Filtered AllowInfo",
			true,
			func(w io.Writer) log.Logger { return log.NewFilter(log.NewTMJSONLoggerNoTS(w), log.AllowInfo()) },
			strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
		{
			"RuleDisabled with Filtered AllowInfo",
			false,
			func(w io.Writer) log.Logger { return log.NewFilter(log.NewTMJSONLoggerNoTS(w), log.AllowInfo()) },
			strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.NewSEM(tc.loggerBackend(&buf))
			addRule(t, logger, log.SemAddress, "a Rule")
			if tc.active {
				goActive(t, logger)
			}

			logger.Debug("here", "this is", "debug log")
			logger.Info("here", "this is", "info log")
			logger.Error("here", "this is", "error log")

			if want, have := tc.want, strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}
		})
	}
}

func TestSemWithWith(t *testing.T) {

	t.Run("Logger with module and user context all fiters allowed",
		func(t *testing.T) {
			var buf bytes.Buffer
			baselogger := log.NewTMJSONLoggerNoTS(&buf)
			filtered := log.NewFilter(baselogger, log.AllowAll())
			semlogger := log.NewSEM(filtered)

			// Create contextual logger from SEM logger
			semWithLogger := semlogger.With("module", "mymod", "user", "Sam")

			// Ensure original output is as expected
			semlogger.Debug("here", "this is", "debug log")
			semlogger.Info("here", "this is", "info log")
			semlogger.Error("here", "this is", "error log")

			want := strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n")

			if have := strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}

			// Check output of context logger is as expected
			buf.Reset()
			semWithLogger.Debug("here", "this is", "debug log")
			semWithLogger.Info("here", "this is", "info log")
			semWithLogger.Error("here", "this is", "error log")

			want = strings.Join([]string{
				`{"_msg":"here","level":"debug","module":"mymod","this is":"debug log","user":"Sam"}`,
				`{"_msg":"here","level":"info","module":"mymod","this is":"info log","user":"Sam"}`,
				`{"_msg":"here","level":"error","module":"mymod","this is":"error log","user":"Sam"}`,
			}, "\n")

			if have := strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}

			// Double check original SEM logger output
			buf.Reset()
			semlogger.Debug("here", "this is", "debug log")
			semlogger.Info("here", "this is", "info log")
			semlogger.Error("here", "this is", "error log")

			want = strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n")
			if have := strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}
		})
}
func TestSemWithCtxFiltering(t *testing.T) {
	testCases := []struct {
		name          string
		active        bool
		enabled       bool
		loggerBackend func(io.Writer) log.Logger
		wantOrig      string
		wantCtx       string
	}{
		{
			"SEM active with '*:Error, consensus:Info'",
			true,
			true,
			func(w io.Writer) log.Logger {
				logger := log.NewTMJSONLoggerNoTS(w)
				return log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "consensus"))
			},
			strings.Join([]string{
				`{"_msg":"here","level":"debug","this is":"debug log"}`,
				`{"_msg":"here","level":"info","this is":"info log"}`,
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
			strings.Join([]string{
				`{"_msg":"here","level":"debug","module":"consensus","this is":"debug log"}`,
				`{"_msg":"here","level":"info","module":"consensus","this is":"info log"}`,
				`{"_msg":"here","level":"error","module":"consensus","this is":"error log"}`,
			}, "\n"),
		},
		{
			"SEM inactive with '*:Error, consensus:Info'",
			false,
			true,
			func(w io.Writer) log.Logger {
				logger := log.NewTMJSONLoggerNoTS(w)
				return log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "consensus"))
			},
			strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
			strings.Join([]string{
				`{"_msg":"here","level":"info","module":"consensus","this is":"info log"}`,
				`{"_msg":"here","level":"error","module":"consensus","this is":"error log"}`,
			}, "\n"),
		},

		{
			"SEM disabled with '*:Error, consensus:Info'",
			false,
			false,
			func(w io.Writer) log.Logger {
				logger := log.NewTMJSONLoggerNoTS(w)
				return log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "consensus"))
			},
			strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n"),
			strings.Join([]string{
				`{"_msg":"here","level":"info","module":"consensus","this is":"info log"}`,
				`{"_msg":"here","level":"error","module":"consensus","this is":"error log"}`,
			}, "\n"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			filtered := tc.loggerBackend(&buf)
			semlogger := log.NewSEM(filtered)

			if tc.enabled {
				addRule(t, semlogger, log.SemAddress, "a Rule")
			}
			if tc.active {
				goActive(t, semlogger)
			}
			semCtxLogger := semlogger.With("module", "consensus")

			// Check SEM Ctx logger
			semCtxLogger.Debug("here", "this is", "debug log")
			semCtxLogger.Info("here", "this is", "info log")
			semCtxLogger.Error("here", "this is", "error log")
			if want, have := tc.wantCtx, strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}

			// Check SEM origin logger
			buf.Reset()
			semlogger.Debug("here", "this is", "debug log")
			semlogger.Info("here", "this is", "info log")
			semlogger.Error("here", "this is", "error log")

			if want, have := tc.wantOrig, strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}
		})
	}
}
