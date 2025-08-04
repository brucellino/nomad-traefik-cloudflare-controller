package types

import "time"

// NodeInfo is a type representing relevant information about a Nomad node.
type NodeInfo struct {
	ID              string // Node ID in Nomad cluster
	Name            string // human-readable name fo the node in the cluster
	PublicIPAddress string // Public IP Address of the node.
	Status          string // Status of the node in the cluster.
}

// DNSRecord represents a DNS record that can be passed to cloudflare API
type DNSRecord struct {
	ID      string
	Name    string // name of the record in Cloudflare
	Type    string // Can be A, AAAA, CNAME, etc
	Content string // the value of the record
	TTL     int    // can also be "auto", but we'll deal with that later.
}

// Event is a Nomad EventStream Event. IT comes as newline separated JSON
type Event struct {
	Type      string
	Timestamp time.Time
	NodeID    string
	JobID     string
	Details   map[string]interface{} // See https://developer.hashicorp.com/nomad/api-docs/events#sample-response for actual event schema
}
