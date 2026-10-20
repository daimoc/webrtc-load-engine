package mediasoup

// The mediasoup JSON types the adapter sends and reads
// (contracts/signaling-messages.md), with only the fields it uses. Codec
// parameters keep the server's JSON values (numbers stay numbers).

// RtpCapabilities are a router's or a bot's RTP capabilities.
type RtpCapabilities struct {
	Codecs           []RtpCodecCapability `json:"codecs"`
	HeaderExtensions []RtpHeaderExtension `json:"headerExtensions"`
}

// RtpCodecCapability is one codec of RtpCapabilities.
type RtpCodecCapability struct {
	Kind                 string         `json:"kind"`
	MimeType             string         `json:"mimeType"`
	PreferredPayloadType int            `json:"preferredPayloadType"`
	ClockRate            int            `json:"clockRate"`
	Channels             int            `json:"channels,omitempty"`
	Parameters           map[string]any `json:"parameters"`
	RtcpFeedback         []RtcpFeedback `json:"rtcpFeedback"`
}

// RtpHeaderExtension is one header extension of RtpCapabilities.
type RtpHeaderExtension struct {
	Kind             string `json:"kind"`
	URI              string `json:"uri"`
	PreferredID      int    `json:"preferredId"`
	PreferredEncrypt bool   `json:"preferredEncrypt"`
	Direction        string `json:"direction,omitempty"`
}

// RtcpFeedback is an RTCP feedback type, e.g. "nack" with parameter "pli".
type RtcpFeedback struct {
	Type      string `json:"type"`
	Parameter string `json:"parameter"`
}

// RtpParameters are a producer's or a consumer's RTP parameters.
type RtpParameters struct {
	MID              string                         `json:"mid,omitempty"`
	Codecs           []RtpCodecParameters           `json:"codecs"`
	HeaderExtensions []RtpHeaderExtensionParameters `json:"headerExtensions"`
	Encodings        []RtpEncodingParameters        `json:"encodings"`
	RTCP             RtcpParameters                 `json:"rtcp"`
}

// RtpCodecParameters is one codec of RtpParameters.
type RtpCodecParameters struct {
	MimeType     string         `json:"mimeType"`
	PayloadType  int            `json:"payloadType"`
	ClockRate    int            `json:"clockRate"`
	Channels     int            `json:"channels,omitempty"`
	Parameters   map[string]any `json:"parameters"`
	RtcpFeedback []RtcpFeedback `json:"rtcpFeedback"`
}

// RtpHeaderExtensionParameters is one header extension of RtpParameters.
type RtpHeaderExtensionParameters struct {
	URI     string `json:"uri"`
	ID      int    `json:"id"`
	Encrypt bool   `json:"encrypt"`
}

// RtpEncodingParameters is one encoding of RtpParameters (one, without
// simulcast).
type RtpEncodingParameters struct {
	SSRC uint32   `json:"ssrc"`
	RTX  *RtxSSRC `json:"rtx,omitempty"`
}

// RtxSSRC is an encoding's retransmission SSRC.
type RtxSSRC struct {
	SSRC uint32 `json:"ssrc"`
}

// RtcpParameters are the RTCP parameters of RtpParameters.
type RtcpParameters struct {
	CNAME       string `json:"cname"`
	ReducedSize bool   `json:"reducedSize"`
}

// IceParameters are a transport's ICE credentials.
type IceParameters struct {
	UsernameFragment string `json:"usernameFragment"`
	Password         string `json:"password"`
	IceLite          bool   `json:"iceLite"`
}

// IceCandidate is one of a transport's ICE candidates.
type IceCandidate struct {
	Foundation string `json:"foundation"`
	Priority   uint32 `json:"priority"`
	Address    string `json:"address"`
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	Type       string `json:"type"`
	TCPType    string `json:"tcpType,omitempty"`
}

// DtlsParameters are a transport's (or the bot's) DTLS role and
// fingerprints.
type DtlsParameters struct {
	Role         string            `json:"role"`
	Fingerprints []DtlsFingerprint `json:"fingerprints"`
}

// DtlsFingerprint is a certificate fingerprint.
type DtlsFingerprint struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// SctpParameters are a transport's SCTP parameters.
type SctpParameters struct {
	Port           int `json:"port"`
	OS             int `json:"OS"`
	MIS            int `json:"MIS"`
	MaxMessageSize int `json:"maxMessageSize"`
}

// TransportInfo is the answer to createWebRtcTransport.
type TransportInfo struct {
	TransportID    string          `json:"transportId"`
	IceParameters  IceParameters   `json:"iceParameters"`
	IceCandidates  []IceCandidate  `json:"iceCandidates"`
	DtlsParameters DtlsParameters  `json:"dtlsParameters"`
	SctpParameters *SctpParameters `json:"sctpParameters,omitempty"`
}

// NewConsumerRequest is the data of the server's newConsumer request.
type NewConsumerRequest struct {
	PeerID         string        `json:"peerId"`
	TransportID    string        `json:"transportId"`
	ConsumerID     string        `json:"consumerId"`
	ProducerID     string        `json:"producerId"`
	Kind           string        `json:"kind"`
	RtpParameters  RtpParameters `json:"rtpParameters"`
	Type           string        `json:"type"`
	ProducerPaused bool          `json:"producerPaused"`
}

// routerCapsResponse is the answer to getRouterRtpCapabilities.
type routerCapsResponse struct {
	RouterRtpCapabilities RtpCapabilities `json:"routerRtpCapabilities"`
}

// joinRequest is the data of join.
type joinRequest struct {
	DisplayName     string          `json:"displayName"`
	Device          device          `json:"device"`
	RtpCapabilities RtpCapabilities `json:"rtpCapabilities"`
}

type device struct {
	Flag    string `json:"flag"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// joinResponse is the answer to join.
type joinResponse struct {
	Peers []struct {
		ID string `json:"id"`
	} `json:"peers"`
}

// createTransportRequest is the data of createWebRtcTransport.
type createTransportRequest struct {
	ForceTCP bool              `json:"forceTcp"`
	AppData  map[string]string `json:"appData"`
}

// connectTransportRequest is the data of connectWebRtcTransport.
type connectTransportRequest struct {
	TransportID    string         `json:"transportId"`
	DtlsParameters DtlsParameters `json:"dtlsParameters"`
}

// produceRequest is the data of produce.
type produceRequest struct {
	TransportID   string            `json:"transportId"`
	Kind          string            `json:"kind"`
	RtpParameters RtpParameters     `json:"rtpParameters"`
	AppData       map[string]string `json:"appData"`
}

// produceResponse is the answer to produce.
type produceResponse struct {
	ProducerID string `json:"producerId"`
}

// consumerRef is the data of consumerClosed and resumeConsumer.
type consumerRef struct {
	ConsumerID string `json:"consumerId"`
}

// peerRef is the data of peerClosed.
type peerRef struct {
	PeerID string `json:"peerId"`
}

// versionNotification is the data of mediasoupVersion.
type versionNotification struct {
	Version string `json:"version"`
}
