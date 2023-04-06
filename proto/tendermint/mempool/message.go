package mempool

import (
	"github.com/cosmos/gogoproto/proto"

	"github.com/cometbft/cometbft/p2p"
)

var _ p2p.Wrapper = &Txs{}

// Wrap implements the p2p Wrapper interface and wraps a mempool message.
func (m *Txs) Wrap() proto.Message {
	mm := &Message{}
	mm.Sum = &Message_Txs{Txs: m}
	return mm
}
