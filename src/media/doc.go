// Package media holds the platform-independent media side of the bots: the
// pre-encoded content they loop, the pacer that sends it to every bot's
// track, and the accounting of received media.
//
// It knows RTP, codecs and media kinds ("audio", "video") only; no signaling
// or platform concept (XMPP, Jingle, source names) belongs here
// (constitution principle II).
package media
