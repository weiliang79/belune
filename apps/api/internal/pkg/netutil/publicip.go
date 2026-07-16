// Package netutil holds small networking helpers shared across the API and
// worker — currently public-IP resolution used by the TLS DNS precheck and the
// Server → Maintenance "Server IP" panel.
package netutil

import "net"

// IsPublicIP reports whether ip is globally routable — i.e. something a user's
// DNS record could legitimately point at.
func IsPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

// DetectEgressIP returns the local address the kernel would use for outbound
// traffic, or "" when it can't be determined or isn't globally routable (behind
// NAT, or on a dev box). No packets are sent: connecting a UDP socket only fixes
// the route, and the local address it picks is the one we would egress from.
// Not cached — callers that poll should cache the result themselves.
func DetectEgressIP() string {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || !IsPublicIP(addr.IP) {
		return ""
	}
	return addr.IP.String()
}
