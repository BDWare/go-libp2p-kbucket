module github.com/bdware/go-libp2p-kbucket

go 1.14

replace github.com/libp2p/go-libp2p-kbucket => ./ // v0.4.3

require (
	github.com/ipfs/go-ipfs-util v0.0.1
	github.com/ipfs/go-log v1.0.4
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20200528110405-70ea36266519
	github.com/libp2p/go-libp2p-core v0.5.4
	github.com/libp2p/go-libp2p-kbucket v0.4.3
	github.com/libp2p/go-libp2p-peerstore v0.2.4
	github.com/minio/sha256-simd v0.1.1
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/multiformats/go-multiaddr-net v0.1.5
	github.com/multiformats/go-multihash v0.0.13
	github.com/stretchr/testify v1.5.1
	github.com/yl2chen/cidranger v1.0.0
)
