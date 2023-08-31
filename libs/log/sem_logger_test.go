package log_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

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

func expectSemActive(t *testing.T, logger log.Logger, active bool) {
	seml, ok := logger.(log.SemLogger)
	require.True(t, ok, "Bad test setup. Expected a SEM logger but it isn't")

	status, err := seml.Status()
	require.NoError(t, err)
	require.Equal(t, status.Active, active,
		fmt.Sprintf("SEM active state mismatch expected %t, have %t",
			active, status.Active))
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
			logger := log.NewSemLogger(tc.loggerBackend(&buf))
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
			semlogger := log.NewSemLogger(filtered)

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
			semlogger := log.NewSemLogger(filtered)

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

func TestRuleUpdateOnCtxLogger(t *testing.T) {

	t.Run("Rule update on context logger",
		func(t *testing.T) {
			var buf bytes.Buffer
			filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf),
				log.AllowError(),
				log.AllowInfoWith("module", "consensus"))
			semlogger := log.NewSemLogger(filtered)

			// Create contextual logger from SEM logger
			semWithCtx := semlogger.With("module", "consensus", "user", "Sam")

			// Ensure original output is as expected
			semlogger.Debug("here", "this is", "debug log")
			semlogger.Info("here", "this is", "info log")
			semlogger.Error("here", "this is", "error log")

			want := strings.Join([]string{
				`{"_msg":"here","level":"error","this is":"error log"}`,
			}, "\n")

			if have := strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}

			// Check output of context logger is as expected
			buf.Reset()
			semWithCtx.Debug("here", "this is", "debug log")
			semWithCtx.Info("here", "this is", "info log")
			semWithCtx.Error("here", "this is", "error log")

			want = strings.Join([]string{
				`{"_msg":"here","level":"info","module":"consensus","this is":"info log","user":"Sam"}`,
				`{"_msg":"here","level":"error","module":"consensus","this is":"error log","user":"Sam"}`,
			}, "\n")

			if have := strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}

			// Add Matching rule to enable activate SEM on non-context SEM
			goActive(t, semlogger)
			buf.Reset()
			expectSemActive(t, semlogger, true)
			expectSemActive(t, semWithCtx, true)

			// Ensure original output is as expected
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

			// Check on ctx logger
			buf.Reset()
			semWithCtx.Debug("here", "this is", "debug log")
			semWithCtx.Info("here", "this is", "info log")
			semWithCtx.Error("here", "this is", "error log")

			want = strings.Join([]string{
				`{"_msg":"here","level":"debug","module":"consensus","this is":"debug log","user":"Sam"}`,
				`{"_msg":"here","level":"info","module":"consensus","this is":"info log","user":"Sam"}`,
				`{"_msg":"here","level":"error","module":"consensus","this is":"error log","user":"Sam"}`,
			}, "\n")

			if have := strings.TrimSpace(buf.String()); want != have {
				t.Errorf("\nwant:\n%s\nhave:\n%s", want, have)
			}

		})
}

func TestSemStates(t *testing.T) {
	t.Run("Entry on unknown rule", func(t *testing.T) {
		var buf bytes.Buffer
		filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf))
		sem := log.NewSemLogger(filtered)
		sem.Entry(log.SemAddress, []byte("myrule"))
		expectSemActive(t, sem, false)
	})
	t.Run("Entry on unknown entity", func(t *testing.T) {
		var buf bytes.Buffer
		filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf))
		sem := log.NewSemLogger(filtered)
		sem.Entry(3, []byte("myrule"))
		expectSemActive(t, sem, false)
	})

}

func TestSemRuleManagement(t *testing.T) {
	t.Run("Add rule", func(t *testing.T) {
		var buf bytes.Buffer
		filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf))
		sem := log.NewSemLogger(filtered)
		rule := "rule1"
		sem.AddRule(log.SemAddress, []byte(rule))
		status, err := sem.Status()
		require.NoError(t, err)
		require.Equal(t, map[log.SemEntity]map[string]bool{log.SemAddress: {rule: false}}, status.Rules)
	})

	t.Run("Delete rules", func(t *testing.T) {
		var buf bytes.Buffer
		filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf))
		sem := log.NewSemLogger(filtered)
		expectedRules := map[log.SemEntity]map[string]bool{
			log.SemAddress:     {"address rule1": false},
			log.SemTransaction: {"transaction rule 1": false}}

		//create rules
		for entity, rules := range expectedRules {
			for rule, _ := range rules {
				sem.AddRule(entity, []byte(rule))
			}
		}

		//cheeck rules
		status, err := sem.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		sem.DeleteAll()
		status, err = sem.Status()
		require.NoError(t, err)
		require.Equal(t, map[log.SemEntity]map[string]bool{}, status.Rules)

	})

	t.Run("Rules on multi SEM loggers", func(t *testing.T) {
		var buf bytes.Buffer
		filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf))
		sem := log.NewSemLogger(filtered)
		semCtx := sem.With().(log.SemLogger)

		expectedRules := map[log.SemEntity]map[string]bool{
			log.SemAddress:     {"address rule1": false},
			log.SemTransaction: {"transaction rule 1": false}}

		// create rules
		for entity, rules := range expectedRules {
			for rule, _ := range rules {
				sem.AddRule(entity, []byte(rule))
			}
		}

		// cheeck rules on initial logger
		status, err := sem.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		// check rules on ctx logger
		status, err = semCtx.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		// Add new rule ont original logger
		entity, rule := log.SemAddress, "new address rule on orig"
		expectedRules[entity][rule] = false
		sem.AddRule(entity, []byte(rule))

		// cheeck rules on initial logger
		status, err = sem.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		// check rules on ctx logger
		status, err = semCtx.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		// Add new rule on ctx logger
		entity, rule = log.SemTransaction, "new transaction rule on ctx"
		expectedRules[entity][rule] = false
		semCtx.AddRule(entity, []byte(rule))

		// cheeck rules on initial logger
		status, err = sem.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		// check rules on ctx logger
		status, err = semCtx.Status()
		require.NoError(t, err)
		require.Equal(t, expectedRules, status.Rules)

		// Delete Rules on ctx logger
		semCtx.DeleteAll()
		status, err = sem.Status()
		require.NoError(t, err)
		require.Equal(t, map[log.SemEntity]map[string]bool{}, status.Rules)

		status, err = semCtx.Status()
		require.NoError(t, err)
		require.Equal(t, map[log.SemEntity]map[string]bool{}, status.Rules)

	})

}

func TestRulesConcurrency(t *testing.T) {

	SemEnter := func(sem log.Logger, entity log.SemEntity, val []byte, wg *sync.WaitGroup) {
		defer wg.Done()
		semEntry := sem
		valEntry := val
		entityEntry := entity
		sem.Info("1 Adding Sem entry")

		log.SemEntry(semEntry, entityEntry, valEntry)
		time.Sleep(time.Millisecond * 5)
		sem.Info("1 Adding Sem exit")
		log.SemExit(semEntry, entityEntry, valEntry)
		time.Sleep(time.Second)
	}

	SemAddEnterExit := func(sem log.Logger, entity log.SemEntity, rule []byte, wg *sync.WaitGroup) {
		defer wg.Done()
		semEntry := sem
		valEntry := rule
		entityEntry := entity
		seml, ok := semEntry.(log.SemLogger)
		if !ok {
			panic("Bad test setup. Expected a SEM logger but it isn't")
		}
		seml.Info("2 Adding rule")
		seml.AddRule(log.SemTransaction, []byte(rule))
		seml.Info("2 Sem entry")
		log.SemEntry(semEntry, entityEntry, valEntry)
		time.Sleep(time.Microsecond)
		seml.Info("2 Sem exit")
		log.SemExit(seml, entityEntry, valEntry)
	}

	SemExit := func(sem log.Logger, entity log.SemEntity, rule []byte, wg *sync.WaitGroup) {
		defer wg.Done()
		semEntry := sem
		valEntry := rule
		entityEntry := entity
		seml, ok := semEntry.(log.SemLogger)
		if !ok {
			panic("Bad test setup. Expected a SEM logger but it isn't")
		}
		time.Sleep(time.Microsecond)
		seml.Info("3 Sem exit")
		log.SemExit(seml, entityEntry, valEntry)
	}

	t.Run("Concurrent update on ruleset", func(t *testing.T) {
		var buf bytes.Buffer
		filtered := log.NewFilter(log.NewTMJSONLoggerNoTS(&buf), log.AllowAll())
		sem := log.NewSemLogger(filtered)
		rule := []byte("my rule")
		entity := log.SemAddress
		sem.AddRule(entity, rule)

		var wg sync.WaitGroup
		wg.Add(15)
		for i := 0; i < 5; i++ {
			go SemEnter(sem.With(), entity, rule, &wg)
		}

		for i := 0; i < 5; i++ {
			go SemAddEnterExit(sem.With(), entity, rule, &wg)
		}

		for i := 0; i < 5; i++ {
			go SemExit(sem.With(), entity, rule, &wg)
		}

		wg.Wait()
	})
}
