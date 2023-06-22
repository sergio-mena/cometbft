package mempool

import (
	"sync"
	"sync/atomic"

	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/types"
)

// mempoolTx is an entry in the mempool
type mempoolTx struct {
	height    int64    // height that this tx had been validated in
	gasWanted int64    // amount of gas this tx states it will require
	tx        types.Tx // validated by the application

	// ids of peers who've sent us this tx (as a map for quick lookups).
	// senders: PeerID -> bool
	senders sync.Map

	// set of node ids (prefixes) to which this tx was sent (by this node or
	// other nodes)
	sentToNodes map[NodeIdPrefix]struct{}
}

// Height returns the height for this transaction
func (memTx *mempoolTx) Height() int64 {
	return atomic.LoadInt64(&memTx.height)
}

func (memTx *mempoolTx) isSender(peerID uint16) bool {
	_, ok := memTx.senders.Load(peerID)
	return ok
}

func (memTx *mempoolTx) addSender(senderID uint16) bool {
	_, added := memTx.senders.LoadOrStore(senderID, true)
	return added
}

func (memTx *mempoolTx) mergeWithSentNodes(nodes []NodeIdPrefix) []NodeIdPrefix {
	peerSet := map[string]struct{}{}
	for p := range memTx.sentToNodes {
		peerSet[p] = struct{}{}
	}
	for _, p := range nodes {
		peerSet[p] = struct{}{}
	}
	result := make([]NodeIdPrefix, len(peerSet))
	for p := range peerSet {
		result = append(result, p)
	}
	return result
}

func (memTx *mempoolTx) wasSentTo(peer p2p.Peer) bool {
	_, ok := memTx.sentToNodes[NodeIdPrefix(peer.ID()[PrefixLength:])]
	return ok
}

func (memTx *mempoolTx) addToNodeSet(peers []NodeIdPrefix) {
	for _, idPrefix := range peers {
		memTx.sentToNodes[NodeIdPrefix(idPrefix)] = struct{}{}
	}
}
