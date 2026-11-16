package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// The production lookups, captured before TestMain swaps in the offline stub.
var (
	realDirectLookup = directLookup
	realSystemLookup = systemLookup
)

// answeringDirectLookup is the stub TestMain installs: 1.1.1.1 answers, so a
// core start leaves the config alone and no test touches the network.
func answeringDirectLookup(context.Context, string, string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.ParseIP("192.0.2.1")}}, nil
}

// stubLookups swaps both lookups for the test and records the log lines.
func stubLookups(t *testing.T, direct func(context.Context, string, string) ([]net.IPAddr, error),
	system func(context.Context, string) ([]net.IPAddr, error)) *[]string {
	t.Helper()
	origDirect, origSystem := directLookup, systemLookup
	t.Cleanup(func() { directLookup, systemLookup = origDirect, origSystem })
	directLookup, systemLookup = direct, system
	return &[]string{}
}

func recorder(lines *[]string) func(string, ...any) {
	var mu sync.Mutex
	return func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		*lines = append(*lines, fmt.Sprintf(format, args...))
	}
}

func ipAddrs(ips ...string) []net.IPAddr {
	out := make([]net.IPAddr, 0, len(ips))
	for _, ip := range ips {
		out = append(out, net.IPAddr{IP: net.ParseIP(ip)})
	}
	return out
}

func timedOut(context.Context, string, string) ([]net.IPAddr, error) {
	return nil, &net.DNSError{Err: "i/o timeout", Name: "vpn.example", IsTimeout: true}
}

func systemAnswers(ips ...string) func(context.Context, string) ([]net.IPAddr, error) {
	return func(context.Context, string) ([]net.IPAddr, error) { return ipAddrs(ips...), nil }
}

// --- the direct resolver dial ----------------------------------------------

// physicalIPv4 finds an interface interfaceDialIP would bind to, or skips.
func physicalIPv4(t *testing.T) (string, net.IP) {
	t.Helper()
	ifs, err := net.Interfaces()
	if err != nil {
		t.Skip("no interfaces")
	}
	for _, iface := range ifs {
		if ip := interfaceDialIP(iface.Name); ip != nil {
			return iface.Name, ip
		}
	}
	t.Skip("no interface with a bindable IPv4")
	return "", nil
}

func withDirectDNSAddr(t *testing.T, addr string) {
	t.Helper()
	orig := directDNSAddr
	t.Cleanup(func() { directDNSAddr = orig })
	directDNSAddr = addr
}

func TestDirectDNSDialLeavesThroughTheInterface(t *testing.T) {
	name, ip := physicalIPv4(t)

	pc, err := net.ListenPacket("udp4", net.JoinHostPort(ip.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	withDirectDNSAddr(t, pc.LocalAddr().String())
	conn, err := directDNSDial(name)(context.Background(), "udp", "ignored:53")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if got := conn.LocalAddr().(*net.UDPAddr).IP; !got.Equal(ip) {
		t.Fatalf("udp source = %v, want %v", got, ip)
	}

	ln, err := net.Listen("tcp4", net.JoinHostPort(ip.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	withDirectDNSAddr(t, ln.Addr().String())
	tc, err := directDNSDial(name)(context.Background(), "tcp", "ignored:53")
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()
	if got := tc.LocalAddr().(*net.TCPAddr).IP; !got.Equal(ip) {
		t.Fatalf("tcp source = %v, want %v", got, ip)
	}
}

func TestDirectDNSDialUnboundWithoutAnInterface(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	withDirectDNSAddr(t, pc.LocalAddr().String())
	conn, err := directDNSDial("")(context.Background(), "udp", "ignored:53")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestRealDirectLookupAsksTheDirectResolver(t *testing.T) {
	// A resolver that never answers: the lookup has to give up on the context,
	// which proves it asked directDNSAddr and not the system resolver.
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	withDirectDNSAddr(t, pc.LocalAddr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := realDirectLookup(ctx, "", "server.example"); err == nil {
		t.Fatal("expected a timeout from a silent resolver")
	}
}

func TestRealSystemLookupResolvesLocalhost(t *testing.T) {
	addrs, err := realSystemLookup(context.Background(), "localhost")
	if err != nil || len(addrs) == 0 {
		t.Fatalf("localhost: %v %v", addrs, err)
	}
}

// --- address filtering -----------------------------------------------------

func TestUsableServerIPsDropsFakeAndLocalAnswers(t *testing.T) {
	addrs := append(ipAddrs("2001:db8::1", "198.18.0.5", "198.19.255.1", "127.0.0.1",
		"0.0.0.0", "::", "::1", "::ffff:203.0.113.7"), net.IPAddr{IP: net.IP{1, 2}})
	got := usableServerIPs(addrs)
	want := []string{"203.0.113.7", "2001:db8::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLookupErrorTextLeavesTheHostnameOut(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &net.DNSError{Err: "no such host", Name: "secret.example"})
	if got := lookupErrorText(err); got != "no such host" {
		t.Fatalf("got %q", got)
	}
	if got := lookupErrorText(errors.New("boom")); got != "boom" {
		t.Fatalf("got %q", got)
	}
}

func TestDirectUnreachableTellsSilenceFromNXDOMAIN(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{&net.DNSError{Err: "no such host", IsNotFound: true}, false},
		{&net.DNSError{Err: "i/o timeout", IsTimeout: true}, true},
		{errors.New("connect: network is unreachable"), true},
	} {
		if got := directUnreachable(tc.err); got != tc.want {
			t.Errorf("%v: got %v", tc.err, got)
		}
	}
}

// --- resolveServerHost -----------------------------------------------------

func TestResolveServerHostLeavesItToTheCoreWhenDirectAnswers(t *testing.T) {
	lines := stubLookups(t, answeringDirectLookup, func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("system resolver asked although 1.1.1.1 answered")
		return nil, nil
	})
	if got, down := resolveServerHost("eth0", "vpn.example", recorder(lines)); got != nil || down {
		t.Fatalf("got %v %v", got, down)
	}
	if len(*lines) != 0 {
		t.Fatalf("logged %v", *lines)
	}
}

func TestResolveServerHostFallsBackToTheSystemResolver(t *testing.T) {
	var gotIface, gotHost string
	lines := stubLookups(t, func(_ context.Context, iface, host string) ([]net.IPAddr, error) {
		gotIface, gotHost = iface, host
		return timedOut(nil, iface, host)
	}, systemAnswers("203.0.113.7"))
	got, down := resolveServerHost("wlan0", "vpn.example", recorder(lines))
	if !reflect.DeepEqual(got, []string{"203.0.113.7"}) || !down {
		t.Fatalf("got %v %v", got, down)
	}
	if gotIface != "wlan0" || gotHost != "vpn.example" {
		t.Fatalf("direct lookup got %q %q", gotIface, gotHost)
	}
	want := "server address: 1.1.1.1 failed (i/o timeout), using the system DNS answer"
	if !reflect.DeepEqual(*lines, []string{want}) {
		t.Fatalf("logged %v", *lines)
	}
}

func TestResolveServerHostGivesUpWhenTheSystemResolverFailsToo(t *testing.T) {
	lines := stubLookups(t, timedOut, func(context.Context, string) ([]net.IPAddr, error) {
		return nil, &net.DNSError{Err: "server misbehaving", Name: "vpn.example"}
	})
	if got, down := resolveServerHost("", "vpn.example", recorder(lines)); got != nil || !down {
		t.Fatalf("got %v %v", got, down)
	}
	want := "server address: 1.1.1.1 failed (i/o timeout), system DNS failed (server misbehaving)"
	if !reflect.DeepEqual(*lines, []string{want}) {
		t.Fatalf("logged %v", *lines)
	}
}

func TestResolveServerHostIgnoresAFakeIPSystemAnswer(t *testing.T) {
	lines := stubLookups(t, func(context.Context, string, string) ([]net.IPAddr, error) {
		return nil, nil
	}, systemAnswers("198.18.0.9"))
	if got, down := resolveServerHost("", "vpn.example", recorder(lines)); got != nil || down {
		t.Fatalf("got %v %v", got, down)
	}
	want := "server address: 1.1.1.1 failed (no addresses), system DNS gave no usable address"
	if !reflect.DeepEqual(*lines, []string{want}) {
		t.Fatalf("logged %v", *lines)
	}
}

// --- config rewriting ------------------------------------------------------

func parseDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

const pinnableConfig = `{
  "dns": {"servers": [
    {"tag": "remote", "type": "https", "server": "1.1.1.1", "detour": "proxy-out"},
    {"tag": "dns-direct", "type": "udp", "server": "1.1.1.1", "bind_interface": "wlan0"}
  ], "final": "remote"},
  "inbounds": [{"type": "socks", "listen_port": 1080}],
  "outbounds": [
    {"type": "vless", "tag": "proxy-out", "server": "vpn.example", "server_port": 443},
    {"type": "direct", "tag": "direct"}
  ]
}`

func TestServerHostOutboundsPicksProxyHostnames(t *testing.T) {
	doc := parseDoc(t, `{"outbounds": [
    "junk",
    {"type": "direct", "server": "skip.example"},
    {"type": "vless", "server": "own.example", "domain_resolver": "remote"},
    {"type": "vless", "server": "203.0.113.7"},
    {"type": "vless"},
    {"type": "vless", "tag": "a", "server": "vpn.example"},
    {"type": "trojan", "tag": "b", "server": "vpn.example"}
  ]}`)
	got := serverHostOutbounds(doc)
	if len(got) != 1 || len(got["vpn.example"]) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestTypedDNSServersRejectsLegacyAndMalformed(t *testing.T) {
	for name, cfg := range map[string]string{
		"no dns":      `{}`,
		"no servers":  `{"dns": {}}`,
		"junk server": `{"dns": {"servers": ["junk"]}}`,
		"legacy":      `{"dns": {"servers": [{"tag": "remote", "address": "https://1.1.1.1/dns-query"}]}}`,
	} {
		if _, ok := typedDNSServers(parseDoc(t, cfg)); ok {
			t.Errorf("%s: accepted", name)
		}
	}
	if servers, ok := typedDNSServers(parseDoc(t, pinnableConfig)); !ok || len(servers) != 2 {
		t.Fatalf("typed config: %v %v", servers, ok)
	}
}

func TestPinServerAddressesPinsTheSystemAnswer(t *testing.T) {
	withLocalResolver(t, true)
	lines := stubLookups(t, timedOut, systemAnswers("203.0.113.7", "2001:db8::1"))
	doc := parseDoc(t, pinnableConfig)
	if !pinServerAddresses(doc, "wlan0", recorder(lines)) {
		t.Fatal("nothing pinned")
	}
	out := doc["outbounds"].([]any)
	if r := out[0].(map[string]any)["domain_resolver"]; r != serverHostsTag {
		t.Fatalf("proxy-out domain_resolver = %v", r)
	}
	if _, has := out[1].(map[string]any)["domain_resolver"]; has {
		t.Fatal("direct outbound touched")
	}
	servers := doc["dns"].(map[string]any)["servers"].([]any)
	hosts := servers[len(servers)-1].(map[string]any)
	want := map[string]any{
		"tag":        serverHostsTag,
		"type":       "hosts",
		"predefined": map[string]any{"vpn.example": []string{"203.0.113.7", "2001:db8::1"}},
	}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("hosts server = %#v", hosts)
	}
	if direct := servers[1]; !reflect.DeepEqual(direct, map[string]any{"tag": "dns-direct", "type": "local"}) {
		t.Fatalf("dns-direct = %#v", direct)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "direct domains use the system DNS") {
		t.Fatalf("logged %v", *lines)
	}
}

func withLocalResolver(t *testing.T, works bool) {
	t.Helper()
	orig := localResolverWorks
	t.Cleanup(func() { localResolverWorks = orig })
	localResolverWorks = works
}

func dnsDirect(doc map[string]any) any {
	return doc["dns"].(map[string]any)["servers"].([]any)[1]
}

func TestPinServerAddressesKeepsDirectDNSOnMacOS(t *testing.T) {
	withLocalResolver(t, false)
	lines := stubLookups(t, timedOut, systemAnswers("203.0.113.7"))
	doc := parseDoc(t, pinnableConfig)
	if !pinServerAddresses(doc, "en0", recorder(lines)) {
		t.Fatal("nothing pinned")
	}
	if dnsDirect(doc).(map[string]any)["type"] != "udp" {
		t.Fatalf("dns-direct = %#v", dnsDirect(doc))
	}
}

func TestPinServerAddressesKeepsDirectDNSWhen1111KnowsNoSuchHost(t *testing.T) {
	withLocalResolver(t, true)
	lines := stubLookups(t, func(context.Context, string, string) ([]net.IPAddr, error) {
		return nil, &net.DNSError{Err: "no such host", IsNotFound: true}
	}, systemAnswers("10.0.0.5"))
	doc := parseDoc(t, pinnableConfig)
	if !pinServerAddresses(doc, "", recorder(lines)) {
		t.Fatal("nothing pinned")
	}
	if dnsDirect(doc).(map[string]any)["type"] != "udp" {
		t.Fatalf("dns-direct = %#v", dnsDirect(doc))
	}
}

func TestPinServerAddressesMovesDirectDNSEvenWithNothingToPin(t *testing.T) {
	withLocalResolver(t, true)
	lines := stubLookups(t, timedOut, systemAnswers("198.18.0.9"))
	doc := parseDoc(t, pinnableConfig)
	if !pinServerAddresses(doc, "", recorder(lines)) {
		t.Fatal("config reported unchanged")
	}
	servers := doc["dns"].(map[string]any)["servers"].([]any)
	if len(servers) != 2 || servers[1].(map[string]any)["type"] != "local" {
		t.Fatalf("servers = %#v", servers)
	}
	if r := doc["outbounds"].([]any)[0].(map[string]any)["domain_resolver"]; r != nil {
		t.Fatalf("proxy-out pinned to %v", r)
	}
}

func TestUseSystemDirectResolverSkipsOtherServers(t *testing.T) {
	servers := []any{"junk", map[string]any{"tag": "remote", "type": "https"}}
	useSystemDirectResolver(servers, func(string, ...any) { t.Fatal("logged") })
	if servers[1].(map[string]any)["type"] != "https" {
		t.Fatalf("servers = %#v", servers)
	}
}

func TestPinServerAddressesLeavesTheConfigAlone(t *testing.T) {
	stubLookups(t, answeringDirectLookup, systemAnswers("203.0.113.7"))
	for name, cfg := range map[string]string{
		"legacy dns":     `{"dns": {"servers": [{"tag": "remote", "address": "1.1.1.1"}]}, "outbounds": [{"type": "vless", "server": "vpn.example"}]}`,
		"no hostname":    `{"dns": {"servers": []}, "outbounds": [{"type": "vless", "server": "203.0.113.7"}]}`,
		"direct answers": pinnableConfig,
	} {
		if pinServerAddresses(parseDoc(t, cfg), "", func(string, ...any) {}) {
			t.Errorf("%s: pinned", name)
		}
	}
}

func TestPinServerAddressesMethod(t *testing.T) {
	stashVersionCache(t)
	c := singBoxCore{}
	lines := stubLookups(t, timedOut, systemAnswers("203.0.113.7"))
	logf := recorder(lines)

	seedVersion(t, "sing-box", "1.11.0")
	if got, err := c.PinServerAddresses([]byte(pinnableConfig), "", logf); err != nil || string(got) != pinnableConfig {
		t.Fatalf("old sing-box: %s %v", got, err)
	}

	seedVersion(t, "sing-box", "1.13.13")
	if _, err := c.PinServerAddresses([]byte("{"), "", logf); err == nil {
		t.Fatal("bad json accepted")
	}
	got, err := c.PinServerAddresses([]byte(pinnableConfig), "", logf)
	if err != nil || !strings.Contains(string(got), `"type": "hosts"`) {
		t.Fatalf("pinned config: %s %v", got, err)
	}

	directLookup = answeringDirectLookup
	if got, err := c.PinServerAddresses([]byte(pinnableConfig), "", logf); err != nil || string(got) != pinnableConfig {
		t.Fatalf("direct answered: %s %v", got, err)
	}
}
