package util

// Chose the name of this file arbitrarily.
// Lifted some code from: https://gist.github.com/nanmu42/9c8139e15542b3c4a1709cb9e9ac61eb

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
)

var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("parse error on %q: %v", cidr, err))
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

func ValidHost(host string) bool {
	if strings.HasSuffix(host, ".onion") && !config.AllowOnion {
		return false
	}

	// Skip all of these checks if we don't need to bother with them.
	if !config.AllowLocal {
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return false
			}

			// Check to see if we landed in any of the private blocks
			for _, block := range privateIPBlocks {
				if block.Contains(ip) {
					return false
				}
			}
		}
	}

	return true
}

func AllEqual[T comparable](values ...T) bool {
	for _, y := range values {
		for _, x := range values {
			if y != x {
				return false
			}
		}
	}

	return true
}

func EqualDomains(u ...string) bool {
	for i, v := range u {
		if url, err := url.Parse(v); err != nil {
			return false
		} else {
			u[i] = url.Host
		}
	}

	return AllEqual(u...)
}
