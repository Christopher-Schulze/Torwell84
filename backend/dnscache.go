package main

import (
	"context"
	"net"
	"sync"
	"time"
)

// dnsCache caches DNS lookups for a short period.
type dnsCache struct {
	mu    sync.Mutex
	cache map[string]cacheEntry
	ttl   time.Duration
	r     *net.Resolver
}

type cacheEntry struct {
	addrs  []string
	expiry time.Time
}

func newDNSCache(ttl time.Duration) *dnsCache {
	return &dnsCache{cache: make(map[string]cacheEntry), ttl: ttl, r: net.DefaultResolver}
}

func (d *dnsCache) LookupHost(host string) ([]string, error) {
	d.mu.Lock()
	if e, ok := d.cache[host]; ok && time.Now().Before(e.expiry) {
		addrs := make([]string, len(e.addrs))
		copy(addrs, e.addrs)
		d.mu.Unlock()
		return addrs, nil
	}
	d.mu.Unlock()

	addrs, err := d.r.LookupHost(context.Background(), host)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	d.cache[host] = cacheEntry{addrs: addrs, expiry: time.Now().Add(d.ttl)}
	d.mu.Unlock()
	return addrs, nil
}
