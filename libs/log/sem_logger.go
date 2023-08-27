package log

import "sync"

// Selective Entity Monitor (SEM) Logger enables rule based log traces

type SemLogger interface {
	Logger
	Entry(SemEntity, []byte)
	Exit(SemEntity, []byte)
	AddRule(SemEntity, []byte)
	DeleteAll()
	Status() (SemStatus, error)
}

// type loglevel byte
type SemEntity uint

const (
	SemTransaction SemEntity = iota + 1
	SemAddress
)

type SemStatus struct {
	Active bool
	Rules  map[SemEntity]map[string]bool
}

type semLogger struct {
	logBackend   Logger
	rules        map[SemEntity]map[string]uint
	active       bool
	enabled      bool
	semRulesLock *sync.RWMutex
}

func (l *semLogger) Error(msg string, kvals ...interface{}) {
	l.logBackend.Error(msg, kvals...)
}

func (l *semLogger) Info(msg string, kvals ...interface{}) {
	if l.forwarding() {
		l.logBackend.Info(msg, kvals...)
	}
}

func (l *semLogger) Debug(msg string, kvals ...interface{}) {
	if l.forwarding() {
		l.logBackend.Debug(msg, kvals...)
	}
}

func (l *semLogger) With(keyvals ...interface{}) Logger {
	var sl *semLogger
	func() {
		l.semRulesLock.RLock()
		defer l.semRulesLock.RUnlock()

		sl = &semLogger{
			logBackend:   l.logBackend.With(keyvals...),
			rules:        l.rules,
			semRulesLock: l.semRulesLock,
		}
	}()

	sl.setActiveState(l.active)
	return sl
}

func (l *semLogger) forwarding() bool {
	// In disabled state (no rules set up) SEM forwards all logs
	if l.isDisabled() {
		return true
	}
	active := l.isActive()
	l.setActiveState(active)
	return active
}

func (l *semLogger) isActive() bool {
	l.semRulesLock.RLock()
	defer l.semRulesLock.RUnlock()
	for _, rules := range l.rules {
		for _, count := range rules {
			if count > 0 {
				return true
			}
		}
	}
	return false
}

func (l *semLogger) isDisabled() bool {
	l.semRulesLock.RLock()
	defer l.semRulesLock.RUnlock()
	return len(l.rules) == 0
}

func (l *semLogger) AddRule(entity SemEntity, value []byte) {
	l.semRulesLock.Lock()
	defer l.semRulesLock.Unlock()

	_, exist := l.rules[entity]
	key := string(value)
	if !exist {
		l.rules[entity] = map[string]uint{key: 0}
	} else {
		_, exist = l.rules[entity][key]
		if !exist {
			l.rules[entity][key] = 0
		}
	}
	l.setEnabled(true)
}

func (l *semLogger) DeleteAll() {
	func() {
		l.semRulesLock.Lock()
		defer l.semRulesLock.Unlock()

		for rule := range l.rules {
			delete(l.rules, rule)
		}
	}()

	l.setEnabled(false)
	l.setActiveState(false)
}

func (l *semLogger) Status() (status SemStatus, err error) {
	status = SemStatus{
		Active: l.isActive(),
		Rules:  map[SemEntity]map[string]bool{},
	}

	l.semRulesLock.RLock()
	defer l.semRulesLock.RUnlock()
	for entity, rules := range l.rules {
		for rule, count := range rules {
			_, exists := status.Rules[entity]
			if !exists {
				status.Rules[entity] = map[string]bool{}
			}
			status.Rules[entity][rule] = count > 0

		}
	}
	return
}

// Set SEM logger state to active
// In active state all log levels are enabled.
// In deactivated state (no matching rule) only error logs are forwarded to logger backend
func (l *semLogger) setActiveState(active bool) {
	// For Filter loggers we need to make sure that no filtering is happening underneath
	next, isFilterLogger := l.logBackend.(*filter)
	if isFilterLogger {
		//Filter backend needs to bypass all logs incase of SEM is enabled
		next.SetSemStatus(!l.isDisabled())
	}
	l.active = active
}

// Disable/Enable SEM logger
// In disabled state (no rules defined) no 'filterring' is done on SEM level and no bypass
// on filter-capable backends should be done
func (l *semLogger) setEnabled(enable bool) {
	l.enabled = enable
	// For Filter loggers we need to make sure that no filtering is happening underneath
	next, isFilterLogger := l.logBackend.(*filter)
	if isFilterLogger {
		next.SetSemStatus(enable)
	}
}

func (l *semLogger) Entry(entity SemEntity, value []byte) {
	if !l.ruleExists(entity, value) {
		return
	}
	if !l.active {
		l.setActiveState(true)
	}
	l.ruleCountIncrease(entity, value)
}

func (l *semLogger) ruleExists(entity SemEntity, rule []byte) bool {
	l.semRulesLock.RLock()
	defer l.semRulesLock.RUnlock()
	_, exist := l.rules[entity][string(rule)]
	return exist
}

func (l *semLogger) ruleGetCount(entity SemEntity, rule []byte) (uint, bool) {
	l.semRulesLock.RLock()
	defer l.semRulesLock.RUnlock()
	count, exist := l.rules[entity][string(rule)]
	return count, exist

}

func (l *semLogger) ruleCountIncrease(entity SemEntity, rule []byte) uint {
	if !l.ruleExists(entity, rule) {
		return 0
	}

	l.semRulesLock.Lock()
	defer l.semRulesLock.Unlock()
	l.rules[entity][string(rule)]++
	return l.rules[entity][string(rule)]
}

func (l *semLogger) ruleCountDecrease(entity SemEntity, rule []byte) uint {
	if !l.ruleExists(entity, rule) {
		return 0
	}

	l.semRulesLock.Lock()
	defer l.semRulesLock.Unlock()

	count := l.rules[entity][string(rule)]
	if count == 0 {
		l.Error("Inconsistent entity guard when exiting from rule ", string(rule))
	} else {
		l.rules[entity][string(rule)]--
	}
	return l.rules[entity][string(rule)]
}

func (l *semLogger) Exit(entity SemEntity, value []byte) {
	if !l.ruleExists(entity, value) {
		return
	}

	l.ruleCountDecrease(entity, value)
	if !l.isActive() {
		l.setActiveState(false)
	}
}

func NewSemLogger(next Logger) SemLogger {
	sem := &semLogger{
		logBackend:   next,
		rules:        map[SemEntity]map[string]uint{},
		semRulesLock: new(sync.RWMutex),
	}
	return sem
}

// Helper to setup SEM entry for a SEM capable logger
func SemEntry(logger Logger, entity SemEntity, val []byte) {
	semLogger, ok := logger.(SemLogger)
	if !ok {
		logger.Debug("Can't use SEM logging. Not a compatible SEM logger")
		return
	}
	semLogger.Entry(entity, val)
}

func SemExit(logger Logger, entity SemEntity, val []byte) {
	semLogger, ok := logger.(SemLogger)
	if !ok {
		logger.Debug("Can't use SEM logging. Not a compatible SEM logger")
		return
	}
	semLogger.Exit(entity, val)
}
