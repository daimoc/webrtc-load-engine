package jitsi

import (
	"encoding/xml"
)

// JingleIQ represents the 'jingle' element within an IQ stanza.
// Namespace: urn:xmpp:jingle:1
type JingleIQ struct {
	XMLName   xml.Name        `xml:"urn:xmpp:jingle:1 jingle"`
	Action    string          `xml:"action,attr"`
	SID       string          `xml:"sid,attr"`
	Initiator string          `xml:"initiator,attr,omitempty"`
	Responder string          `xml:"responder,attr,omitempty"`
	Content   []JingleContent `xml:"content"`
	Reason    *JingleReason   `xml:"reason,omitempty"`
}

// JingleContent represents the 'content' element.
type JingleContent struct {
	Creator     string             `xml:"creator,attr"`
	Name        string             `xml:"name,attr"`
	Description *JingleDescription `xml:"description,omitempty"`
	Transport   *JingleTransport   `xml:"transport,omitempty"`
}

// JingleDescription represents the 'description' element.
// Namespace: urn:xmpp:jingle:apps:rtp:1 (for RTP)
type JingleDescription struct {
	XMLName      xml.Name      `xml:"urn:xmpp:jingle:apps:rtp:1 description"`
	Media        string        `xml:"media,attr"`
	SSRC         string        `xml:"ssrc,attr,omitempty"` // Example attribute
	PayloadTypes []PayloadType `xml:"payload-type"`
	RtcpMux      *RtcpMux      `xml:"rtcp-mux,omitempty"`
	// Add other RTP description fields as needed (e.g., encryption)
}

// RtcpMux represents the 'rtcp-mux' element.
type RtcpMux struct {
	XMLName xml.Name `xml:"rtcp-mux"`
}

// PayloadType represents the 'payload-type' element.
type PayloadType struct {
	ID         string      `xml:"id,attr"`
	Name       string      `xml:"name,attr"`
	ClockRate  string      `xml:"clockrate,attr,omitempty"`
	Channels   string      `xml:"channels,attr,omitempty"`
	Parameters []Parameter `xml:"parameter"`
	RtcpFb     []RtcpFb    `xml:"rtcp-fb"`
}

// Parameter represents a generic parameter.
type Parameter struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr,omitempty"`
}

// RtcpFb represents rtcp-fb element
type RtcpFb struct {
	Type    string `xml:"type,attr"`
	Subtype string `xml:"subtype,attr,omitempty"`
}

// JingleTransport represents the 'transport' element.
// Namespace: urn:xmpp:jingle:transports:ice-udp:1 (for ICE-UDP)
type JingleTransport struct {
	XMLName     xml.Name          `xml:"urn:xmpp:jingle:transports:ice-udp:1 transport"`
	Ufrag       string            `xml:"ufrag,attr,omitempty"`
	Pwd         string            `xml:"pwd,attr,omitempty"`
	Candidates  []JingleCandidate `xml:"candidate"`
	Fingerprints []Fingerprint    `xml:"fingerprint"`
}

// Fingerprint represents the 'fingerprint' element (XEP-0320).
type Fingerprint struct {
	XMLName xml.Name `xml:"fingerprint"`
	Hash    string   `xml:"hash,attr"`
	Setup   string   `xml:"setup,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

// JingleCandidate represents a candidate in the transport.
type JingleCandidate struct {
    Component  string `xml:"component,attr"`
    Foundation string `xml:"foundation,attr"`
    Generation string `xml:"generation,attr"`
    ID         string `xml:"id,attr"`
    IP         string `xml:"ip,attr"`
    Network    string `xml:"network,attr,omitempty"`
    Port       string `xml:"port,attr"`
    Priority   string `xml:"priority,attr"`
    Protocol   string `xml:"protocol,attr"`
    Type       string `xml:"type,attr"`
    RelAddr    string `xml:"rel-addr,attr,omitempty"`
    RelPort    string `xml:"rel-port,attr,omitempty"`
}

// JingleReason represents the 'reason' element.
type JingleReason struct {
    Condition string `xml:",anyname"` // Simplified: matches the first child element
    Text      string `xml:"text,omitempty"`
}