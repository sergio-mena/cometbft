package core

import (
	"errors"

	"github.com/tendermint/tendermint/libs/log"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// SemAddRule adds a rule referring to an entity to be monitored
// More: https://docs.cometbft.com/v0.34/rpc/#/Sem/sem_add_rule
func SemAddRule(ctx *rpctypes.Context, entityType uint, value []byte) (*ctypes.ResultSemAddRule, error) {
	if entityType == 0 {
		return nil, errors.New("zero value not allowed as entity type")
	}

	log.SemAddRule(env.Logger, log.SemEntity(entityType), value)
	return &ctypes.ResultSemAddRule{}, nil
}

func SemDeleteAll(ctx *rpctypes.Context) (*ctypes.ResultSemDeleteAll, error) {
	log.SemDeleteAll(env.Logger)
	return &ctypes.ResultSemDeleteAll{}, nil
}

func SemStatus(ctx *rpctypes.Context) (*ctypes.ResultSemStatus, error) {

	status, err := log.SemGetStatus(env.Logger)
	if err != nil {
		return nil, err
	}

	res := &ctypes.ResultSemStatus{Rules: make([]ctypes.SemRule, 0)}
	for entityType, values := range status.Rules {
		for entityValue, _ := range values {
			newRule := ctypes.SemRule{
				EntityType: uint(entityType),
				Value:      []byte(entityValue),
			}
			res.Rules = append(res.Rules, newRule)
		}
	}
	return res, nil
}
