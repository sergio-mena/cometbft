package log

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
type SemEntity int

const (
	SemTransaction SemEntity = iota + 1
	SemAddress
)

type SemStatus struct {
	Active bool
	Rules  map[SemEntity]map[string]bool
}

type semLogger struct {
	logBackend Logger
	rules      map[SemEntity]map[string]uint
	active     bool
	enabled    bool
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
	return len(l.rules) == 0
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
	sl := new(semLogger)
	sl.logBackend = l.logBackend.With(keyvals...)
	sl.rules = l.rules
	sl.setActiveState(l.active)
	return sl
}

func (l *semLogger) AddRule(entity SemEntity, value []byte) {
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
	for rule := range l.rules {
		delete(l.rules, rule)
	}
	l.setEnabled(false)
	l.setActiveState(false)
}

func (l *semLogger) Status() (status SemStatus, err error) {
	status = SemStatus{
		Active: l.forwarding(),
		Rules:  map[SemEntity]map[string]bool{},
	}
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
		next.SetSemStatus(active)
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
	_, exist := l.rules[entity][string(value)]
	if !exist {
		return
	}
	if !l.active {
		l.setActiveState(true)
	}
	l.rules[entity][string(value)]++
}

func (l *semLogger) Exit(entity SemEntity, value []byte) {
	_, exist := l.rules[entity][string(value)]
	if !exist {
		return
	}

	l.rules[entity][string(value)]-- //TODO log on underflow
	for _, ruleset := range l.rules {
		for _, matchCount := range ruleset {
			if matchCount > 0 {
				return
			}

		}
	}
	l.setActiveState(false)
}

func NewSEM(next Logger) SemLogger {
	sem := &semLogger{
		logBackend: next,
		rules:      map[SemEntity]map[string]uint{},
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
