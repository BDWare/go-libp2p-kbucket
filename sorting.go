// Copyright for portions of this fork are held by [Protocol Labs, Inc., 2016] as
// part of the original go-libp2p-kad-dht project. All other copyright for
// this fork are held by [The BDWare Authors, 2020]. All rights reserved.
// Use of this source code is governed by MIT license that can be
// found in the LICENSE file.

package kbucket

import (
	"container/list"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

// sorter sorts peers using some strategy
type sorter interface {
	Peers() []peer.ID
	Len() int
	Swap(a, b int)
	Less(a, b int) bool
	appendPeer(p peer.ID, pDhtId ID)
	appendPeersFromList(l *list.List)
	sort()
}

// peerDistanceSorter implements sort.Interface to sort peers by xor distance
type peerDistanceSorter struct {
	peers     []peer.ID
	distances []ID
	target    ID
}

func (pds *peerDistanceSorter) Peers() []peer.ID { return pds.peers }

func (pds *peerDistanceSorter) Len() int { return len(pds.peers) }
func (pds *peerDistanceSorter) Swap(a, b int) {
	pds.peers[a], pds.peers[b] = pds.peers[b], pds.peers[a]
	pds.distances[a], pds.distances[b] = pds.distances[b], pds.distances[a]
}
func (pds *peerDistanceSorter) Less(a, b int) bool {
	return pds.distances[a].less(pds.distances[b])
}

// Append the peer.ID to the sorter's slice. It may no longer be sorted.
func (pds *peerDistanceSorter) appendPeer(p peer.ID, pDhtId ID) {
	pds.peers = append(pds.peers, p)
	pds.distances = append(pds.distances, xor(pds.target, pDhtId))
}

// Append the peer.ID values in the list to the sorter's slice. It may no longer be sorted.
func (pds *peerDistanceSorter) appendPeersFromList(l *list.List) {
	for e := l.Front(); e != nil; e = e.Next() {
		pds.appendPeer(e.Value.(*PeerInfo).Id, e.Value.(*PeerInfo).dhtId)
	}
}

func (pds *peerDistanceSorter) sort() {
	sort.Sort(pds)
}

/* #BDWare */

// A helper struct to sort peers by their distance to the local node,
// and the latency between the local node and each peer
type peerDistanceCplLatency struct {
	distance ID
	// Is the local peer
	isLocal bool
	cpl     int
	latency time.Duration
}

// peerDistanceLatencySorter implements sort.Interface to sort peers by xor distance and latency
// Reference: Wang Xiang, "Design and Implementation of Low-latency P2P Network for Graph-Based Distributed Ledger" 2020.
// 202006 向往 - 面向图式账本的低延迟P2P网络的设计与实现.pdf
type peerDistanceLatencySorter struct {
	peers   []peer.ID
	infos   []peerDistanceCplLatency
	metrics peerstore.Metrics
	local   ID
	target  ID
	// Estimated average number of bits improved per step.
	avgBitsImprovedPerStep float64
	// Estimated average numbter of round trip required per step.
	// Examples:
	// For TCP+TLS1.3, avgRoundTripPerStep = 4
	// For QUIC, avgRoundTripPerStep = 2
	avgRoundTripPerStep float64
	// Average latency of peers in nanoseconds
	avgLatency int64
}

func (pdls *peerDistanceLatencySorter) Peers() []peer.ID { return pdls.peers }

func (pdls *peerDistanceLatencySorter) Len() int { return len(pdls.peers) }
func (pdls *peerDistanceLatencySorter) Swap(a, b int) {
	pdls.peers[a], pdls.peers[b] = pdls.peers[b], pdls.peers[a]
	pdls.infos[a], pdls.infos[b] = pdls.infos[b], pdls.infos[a]
}
func (pdls *peerDistanceLatencySorter) Less(a, b int) bool {
	// only a is the local peer, a < b
	if pdls.infos[a].isLocal && !pdls.infos[b].isLocal {
		return true
	}
	// b is the local peer, a ≥ b
	if pdls.infos[b].isLocal {
		return false
	}

	// If avgLatency > 0, compare our priority score
	if pdls.avgLatency > 0 {
		deltaCpl := float64(pdls.infos[a].cpl - pdls.infos[b].cpl)
		deltaLatency := float64(pdls.infos[a].latency.Nanoseconds() - pdls.infos[b].latency.Nanoseconds())
		priority := deltaCpl*pdls.avgRoundTripPerStep/pdls.avgBitsImprovedPerStep - deltaLatency/float64(pdls.avgLatency)
		return priority > 0
	}

	// Otherwise, fall back to comparing distances to target
	return pdls.infos[a].distance.less(pdls.infos[b].distance)
}

// Append the peer.ID to the sorter's slice. It may no longer be sorted.
func (pdls *peerDistanceLatencySorter) appendPeer(p peer.ID, pDhtId ID) {
	pdls.peers = append(pdls.peers, p)
	pdls.infos = append(pdls.infos, peerDistanceCplLatency{
		distance: xor(pdls.target, pDhtId),
		isLocal:  pDhtId.equal(pdls.local),
		cpl:      CommonPrefixLen(pdls.target, pDhtId),
		latency:  pdls.metrics.LatencyEWMA(p),
	})
}

// Append the peer.ID values in the list to the sorter's slice. It may no longer be sorted.
func (pdls *peerDistanceLatencySorter) appendPeersFromList(l *list.List) {
	for e := l.Front(); e != nil; e = e.Next() {
		pdls.appendPeer(e.Value.(*PeerInfo).Id, e.Value.(*PeerInfo).dhtId)
	}
}

func (pdls *peerDistanceLatencySorter) sort() {
	sort.Sort(pdls)
}

// Sort the given peers by their ascending distance from the target.
// If rt.considerLatency is true it will consider both xor distance and latency.
// A new slice is returned.
func (rt *RoutingTable) SortClosestPeers(peers []peer.ID, target ID) []peer.ID {
	var s sorter
	// #BDWare
	if rt.considerLatency {
		s = &peerDistanceLatencySorter{
			peers:                  make([]peer.ID, 0, len(peers)),
			infos:                  make([]peerDistanceCplLatency, 0, len(peers)),
			metrics:                rt.metrics,
			local:                  rt.local,
			target:                 target,
			avgBitsImprovedPerStep: rt.avgBitsImprovedPerStep,
			avgRoundTripPerStep:    rt.avgRoundTripPerStep,
			avgLatency:             rt.avgLatency(),
		}
	} else {
		s = &peerDistanceSorter{
			peers:     make([]peer.ID, 0, len(peers)),
			distances: make([]ID, 0, len(peers)),
			target:    target,
		}
	}
	for _, p := range peers {
		s.appendPeer(p, ConvertPeerID(p))
	}
	s.sort()
	out := make([]peer.ID, s.Len())
	copy(out, s.Peers())
	return out
}
