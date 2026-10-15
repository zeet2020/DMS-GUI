package main

import (
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// castHost is what the mDNS Cast probe revealed about one Google
// Cast-enabled device (Chromecast, Android TV with Cast built in, Google/Nest
// speakers) straight from its TXT record.
type castHost struct {
	Model string
	Name  string
}

// castService is the well-known mDNS service type Cast devices register.
const castService = "_googlecast._tcp.local."

// mdnsCastDiscover sends a single targeted mDNS query for the Cast service
// and collects replies for the given window. Only reaches Chromecast-built-in
// devices — not a general mDNS browse.
func mdnsCastDiscover(window time.Duration) map[string]castHost {
	results := map[string]castHost{}

	query, err := buildCastQuery()
	if err != nil {
		return results
	}

	addr, err := net.ResolveUDPAddr("udp4", "224.0.0.251:5353")
	if err != nil {
		return results
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return results
	}
	defer conn.Close()

	if _, err := conn.WriteToUDP(query, addr); err != nil {
		return results
	}

	_ = conn.SetReadDeadline(time.Now().Add(window))
	buf := make([]byte, 4096)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // deadline reached
		}
		var msg dnsmessage.Message
		if err := msg.Unpack(buf[:n]); err != nil {
			continue
		}
		host := castHostFromAnswers(msg.Answers)
		if host.Model != "" || host.Name != "" {
			results[from.IP.String()] = host
		}
	}
	return results
}

func buildCastQuery() ([]byte, error) {
	name, err := dnsmessage.NewName(castService)
	if err != nil {
		return nil, err
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	q := dnsmessage.Question{Name: name, Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}
	if err := b.Question(q); err != nil {
		return nil, err
	}
	return b.Finish()
}

// castHostFromAnswers reads the "md" (model) and "fn" (friendly name) keys
// out of any TXT record in the reply.
func castHostFromAnswers(answers []dnsmessage.Resource) castHost {
	var host castHost
	for _, ans := range answers {
		txt, ok := ans.Body.(*dnsmessage.TXTResource)
		if !ok {
			continue
		}
		for _, kv := range txt.TXT {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			switch k {
			case "md":
				host.Model = v
			case "fn":
				host.Name = v
			}
		}
	}
	return host
}
