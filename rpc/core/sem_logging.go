package core

import (
	"errors"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// TODO: This should disappear and use sem backend instead
var rules = []ctypes.SemRule{}

// SemAddRule adds a rule referring to an entity to be monitored
// More: https://docs.cometbft.com/v0.34/rpc/#/Sem/sem_add_rule
func SemAddRule(ctx *rpctypes.Context, entityType uint, value []byte) (*ctypes.ResultSemAddRule, error) {
	if entityType == 0 {
		return nil, errors.New("zero value not allowed as entity type")
	}

	newRule := ctypes.SemRule{EntityType: entityType, Value: value}
	rules = append(rules, newRule)

	return &ctypes.ResultSemAddRule{}, nil
}

func SemDeleteAll(ctx *rpctypes.Context) (*ctypes.ResultSemDeleteAll, error) {
	rules = []ctypes.SemRule{}
	return &ctypes.ResultSemDeleteAll{}, nil
}

func SemStatus(ctx *rpctypes.Context) (*ctypes.ResultSemStatus, error) {
	resRules := make([]ctypes.SemRule, len(rules))
	copy(resRules, rules)
	return &ctypes.ResultSemStatus{Rules: resRules}, nil
}
