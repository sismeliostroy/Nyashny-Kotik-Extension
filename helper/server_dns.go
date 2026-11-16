package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"time"
)

// The extension has sing-box resolve the proxy server's own hostname through a
// plain UDP query to 1.1.1.1 (the `dns-direct` server): the system resolver is
// broken on macOS for sing-box's `local` type, and a system VPN with fake-ip
// DNS answers with an address the bound outbound cannot reach. Some networks
// drop DNS to anything but their own resolver, though, and there the query
// times out, the outbound never dials, and the tunnel never forms -- while
// every other client on the machine, using the system resolver, connects.
//
// So the helper asks 1.1.1.1 first, the same way the core would. When it
// answers, the config stays as it is. When it does not, the helper asks the
// system resolver, keeps the answer only if it is a real address, and pins it
// for the proxy outbound through a `hosts` DNS server. 1.1.1.1 stays first
// because a provider's resolver may hand back a stub address for a blocked
// domain, which is the very case a proxy server's hostname is likely to be in.
// A 1.1.1.1 that does not answer at all also moves `dns-direct` itself to the
// system resolver, or every domain the rules send direct fails the same way.

// serverHostsTag names the DNS server that carries the pinned addresses.
const serverHostsTag = "kotik-server-hosts"

// serverLookupTimeout bounds each of the two lookups. 1.1.1.1 answers in tens
// of milliseconds where it is reachable at all.
var serverLookupTimeout = 3 * time.Second

// directDNSAddr is the resolver the extension's `dns-direct` server queries.
// A seam for tests, never reassigned in production.
var directDNSAddr = "1.1.1.1:53"

// fakeIPRange is the benchmark block TUN-mode VPNs hand out as fake-ip answers.
var fakeIPRange = netip.MustParsePrefix("198.18.0.0/15")

// directDNSDial dials the direct resolver from the interface's own address,
// the way probeDialer steps out of a TUN, so the answer reflects what the
// core's bound `dns-direct` server would get.
func directDNSDial(iface string) func(ctx context.Context, network, _ string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		d := &net.Dialer{}
		if local := interfaceDialIP(iface); local != nil {
			// The Go resolver retries a truncated answer over TCP, and a TCP
			// dial wants a TCP source address.
			if network == "tcp" {
				d.LocalAddr = &net.TCPAddr{IP: local}
			} else {
				d.LocalAddr = &net.UDPAddr{IP: local}
			}
		}
		return d.DialContext(ctx, network, directDNSAddr)
	}
}

// directLookup is a seam for tests.
var directLookup = func(ctx context.Context, iface, host string) ([]net.IPAddr, error) {
	r := &net.Resolver{PreferGo: true, Dial: directDNSDial(iface)}
	return r.LookupIPAddr(ctx, host)
}

// systemLookup is a seam for tests.
var systemLookup = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// usableServerIPs keeps the addresses a bound outbound can dial, IPv4 first to
// match the extension's prefer_ipv4 strategy. Fake-ip, loopback and unspecified
// answers come from a VPN or a filter, not from the server's zone.
func usableServerIPs(addrs []net.IPAddr) []string {
	var v4, v6 []string
	for _, a := range addrs {
		ip, ok := netip.AddrFromSlice(a.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		if fakeIPRange.Contains(ip) || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		if ip.Is4() {
			v4 = append(v4, ip.String())
		} else {
			v6 = append(v6, ip.String())
		}
	}
	return append(v4, v6...)
}

// lookupErrorText keeps a lookup failure readable in the extension's log
// without the hostname, which problem reports must not carry.
func lookupErrorText(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Err
	}
	return err.Error()
}

// directUnreachable tells a resolver that never answered from one that did and
// knew no such name: only the first says the network keeps 1.1.1.1 out.
func directUnreachable(err error) bool {
	var dnsErr *net.DNSError
	return err != nil && !(errors.As(err, &dnsErr) && dnsErr.IsNotFound)
}

// resolveServerHost returns the addresses to pin for host, or nil when the
// core should resolve it itself: 1.1.1.1 answered, or the system resolver had
// nothing usable to offer either. down reports that 1.1.1.1 did not answer.
func resolveServerHost(iface, host string, logf func(string, ...any)) (ips []string, down bool) {
	ctx, cancel := context.WithTimeout(context.Background(), serverLookupTimeout)
	addrs, err := directLookup(ctx, iface, host)
	cancel()
	if err == nil && len(addrs) > 0 {
		return nil, false
	}
	down = directUnreachable(err)
	direct := "no addresses"
	if err != nil {
		direct = lookupErrorText(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), serverLookupTimeout)
	defer cancel()
	addrs, err = systemLookup(ctx, host)
	if err != nil {
		logf("server address: 1.1.1.1 failed (%s), system DNS failed (%s)", direct, lookupErrorText(err))
		return nil, down
	}
	ips = usableServerIPs(addrs)
	if len(ips) == 0 {
		logf("server address: 1.1.1.1 failed (%s), system DNS gave no usable address", direct)
		return nil, down
	}
	logf("server address: 1.1.1.1 failed (%s), using the system DNS answer", direct)
	return ips, down
}

// localResolverWorks gates moving `dns-direct` to sing-box's `local` server.
// On macOS that server reads a placeholder /etc/resolv.conf and fails every
// lookup, which is why the extension asks 1.1.1.1 in the first place.
var localResolverWorks = runtime.GOOS != "darwin"

// useSystemDirectResolver points `dns-direct` at the system resolver once
// 1.1.1.1 is known not to answer. The proxy server is pinned already; what is
// left on `dns-direct` is every domain the routing rules send direct, and the
// system resolver is the one a direct connection goes out next to. It is left
// unbound for that reason: a direct connection is not bound either.
func useSystemDirectResolver(servers []any, logf func(string, ...any)) {
	for i, s := range servers {
		if m, _ := s.(map[string]any); m["tag"] == "dns-direct" {
			servers[i] = map[string]any{"tag": "dns-direct", "type": "local"}
			logf("direct DNS: 1.1.1.1 does not answer here, direct domains use the system DNS")
		}
	}
}

// serverHostOutbounds maps each hostname a proxy outbound dials to the
// outbounds dialing it. Outbounds with a resolver of their own, or with an IP
// for a server, have nothing to pin.
func serverHostOutbounds(doc map[string]any) map[string][]map[string]any {
	byHost := map[string][]map[string]any{}
	obs, _ := doc["outbounds"].([]any)
	for _, ob := range obs {
		m, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); !isProxyOutboundType(t) {
			continue
		}
		if _, has := m["domain_resolver"]; has {
			continue
		}
		host, _ := m["server"].(string)
		if host == "" || net.ParseIP(host) != nil {
			continue
		}
		byHost[host] = append(byHost[host], m)
	}
	return byHost
}

// typedDNSServers returns the config's DNS server list when every entry uses
// the 1.12 typed schema. A legacy entry means a legacy sing-box, which knows
// neither the `hosts` server nor the outbound `domain_resolver` field.
func typedDNSServers(doc map[string]any) ([]any, bool) {
	dns, ok := doc["dns"].(map[string]any)
	if !ok {
		return nil, false
	}
	servers, ok := dns["servers"].([]any)
	if !ok {
		return nil, false
	}
	for _, s := range servers {
		m, ok := s.(map[string]any)
		if !ok {
			return nil, false
		}
		if t, _ := m["type"].(string); t == "" {
			return nil, false
		}
	}
	return servers, true
}

// pinServerAddresses resolves every proxy server hostname in doc as described
// above and reports whether it pinned any. The lookups run side by side: the
// latency tester's config holds one outbound per server in the list.
func pinServerAddresses(doc map[string]any, iface string, logf func(string, ...any)) bool {
	servers, ok := typedDNSServers(doc)
	if !ok {
		return false
	}
	byHost := serverHostOutbounds(doc)
	if len(byHost) == 0 {
		return false
	}
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		pinned = map[string]any{}
		down   bool
	)
	for host := range byHost {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ips, hostDown := resolveServerHost(iface, host, logf)
			mu.Lock()
			defer mu.Unlock()
			down = down || hostDown
			if ips != nil {
				pinned[host] = ips
			}
		}()
	}
	wg.Wait()
	if down && localResolverWorks {
		useSystemDirectResolver(servers, logf)
	}
	if len(pinned) == 0 {
		return down && localResolverWorks
	}
	for host := range pinned {
		for _, m := range byHost[host] {
			m["domain_resolver"] = serverHostsTag
		}
	}
	doc["dns"].(map[string]any)["servers"] = append(servers, map[string]any{
		"tag":        serverHostsTag,
		"type":       "hosts",
		"predefined": pinned,
	})
	return true
}

// serverAddressPinner is the optional Core step behind PinServerAddresses. Only
// sing-box has it: xray and mihomo carry no DNS section and resolve the server
// through the system resolver already.
type serverAddressPinner interface {
	PinServerAddresses(raw []byte, iface string, logf func(string, ...any)) ([]byte, error)
}

// PinServerAddresses applies pinServerAddresses to a serialized config. It runs
// after InjectBindInterface, which would otherwise write dial fields into the
// `hosts` server, a type that takes none. sing-box before 1.12 has no `hosts`
// server, so its config is returned as-is.
func (c singBoxCore) PinServerAddresses(raw []byte, iface string, logf func(string, ...any)) ([]byte, error) {
	if !singboxAtLeast(c, 1, 12) {
		return raw, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if !pinServerAddresses(doc, iface, logf) {
		return raw, nil
	}
	return json.MarshalIndent(doc, "", "  ")
}
