package flare

import (
	"fmt"
	"net"
	"strings"
)

// default consulted by clientip when peer is a trusted proxy
var defaultRemoteIPHeaders = []string{"X-Forwarded-For", "X-Real-IP"}

// set cidrs/bare ips allowed to supply forwarded-client headers
// empty means clientip always returns sock peer
func (e *Engine) SetTrustedProxies(proxies ...string) error {
	e.mustWritable()
	nets := make([]*net.IPNet, 0, len(proxies))
	for _, p := range proxies {
		n, err := parseProxyNet(p)
		if err != nil {
			return err
		}
		nets = append(nets, n)
	}
	e.trustedProxies = nets
	return nil
}

// overrides header list used when peer is trusted
// defaults restored with nothing passed
func (e *Engine) SetRemoteIPHeaders(headers ...string) {
	e.mustWritable()
	if len(headers) == 0 {
		e.remoteIPHeaders = nil
		return
	}
	e.remoteIPHeaders = append([]string(nil), headers...)
}

func parseProxyNet(p string) (*net.IPNet, error) {
	if !strings.Contains(p, "/") {
		ip := net.ParseIP(p)
		if ip == nil {
			return nil, fmt.Errorf("flare: invalid proxy IP %q", p)
		}
		if ip.To4() != nil {
			p = ip.String() + "/32"
		} else {
			p = ip.String() + "/128"
		}
	}
	_, n, err := net.ParseCIDR(p)
	if err != nil {
		return nil, fmt.Errorf("flare: invalid proxy CIDR %q: %w", p, err)
	}
	return n, nil
}

func (e *Engine) isTrustedProxy(ip net.IP) bool {
	if ip == nil || len(e.trustedProxies) == 0 {
		return false
	}
	for _, n := range e.trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// returns client address
// if socket peer is listed in SetTrustedProxies, headers from
// SetRemoteIPHeaders are consulted
// w/o anything configured headers are ignored and the peer address is returned
func (c *Ctx) ClientIP() string {
	remote := c.RemoteIP()
	if c.engine == nil || !c.engine.isTrustedProxy(remote) {
		if remote == nil {
			return ""
		}
		return remote.String()
	}
	headers := c.engine.remoteIPHeaders
	if len(headers) == 0 {
		headers = defaultRemoteIPHeaders
	}
	for _, h := range headers {
		if ip := clientIPFromHeader(c.GetHeader(h), c.engine); ip != "" {
			return ip
		}
	}
	return remote.String()
}

func clientIPFromHeader(raw string, e *Engine) string {
	if raw == "" {
		return ""
	}
	// r->l w/o split
	for end := len(raw); end > 0; {
		start := end
		for start > 0 && raw[start-1] != ',' {
			start--
		}
		if ip := net.ParseIP(strings.TrimSpace(raw[start:end])); ip != nil && !e.isTrustedProxy(ip) {
			return ip.String()
		}
		if start == 0 {
			break
		}
		end = start - 1
	}
	// all trusted/junk on right takes first valid from left
	for start := 0; start < len(raw); {
		end := start
		for end < len(raw) && raw[end] != ',' {
			end++
		}
		if ip := net.ParseIP(strings.TrimSpace(raw[start:end])); ip != nil {
			return ip.String()
		}
		if end >= len(raw) {
			break
		}
		start = end + 1
	}
	return ""
}
