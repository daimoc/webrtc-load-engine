package livekit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"webrtc-load-engine/src/media"

	"webrtc-load-engine/src/platform"
)

// connectTimeout bounds a bot's join: dial, JoinResponse, publish and the
// first subscriber offer (a variable for tests).
var connectTimeout = 20 * time.Second

// Adapter joins bots to a LiveKit room (research R1–R9). It implements
// platform.PlatformAdapter and platform.PublishSessionAdapter.
type Adapter struct {
	logger *slog.Logger
	cfg    Config

	mu       sync.Mutex
	sessions map[string]*session // By peer ID
}

var (
	_ platform.PlatformAdapter       = (*Adapter)(nil)
	_ platform.PublishSessionAdapter = (*Adapter)(nil)
)

// NewAdapter returns an adapter for the LiveKit server and room of cfg.
func NewAdapter(logger *slog.Logger, cfg Config) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{logger: logger, cfg: cfg, sessions: map[string]*session{}}
}

// Config returns the adapter's configuration.
func (a *Adapter) Config() Config { return a.cfg }

// PublishSession implements platform.PublishSessionAdapter: LiveKit
// receives a participant's media on a session the participant offers.
func (a *Adapter) PublishSession() bool { return true }

// Connect joins peerID to the room and blocks until it can send and receive
// (FR-003): JoinResponse received, its tracks published (sending bots) and
// the server's first subscriber offer answered. On error, everything is
// closed.
func (a *Adapter) Connect(ctx context.Context, peerID string, h platform.SignalingHandler) error {
	logger := a.logger.With(slog.String("peer_id", peerID))
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	token, err := tokenFor(a.cfg, peerID, time.Now())
	if err != nil {
		return fmt.Errorf("livekit join failed: %w", err)
	}
	u := dialURL(a.cfg.URL, token)
	logger.Info("LiveKit adapter: connecting peer", slog.String("url", redactURL(u)))
	conn, err := dial(ctx, u)
	if err != nil {
		return fmt.Errorf("livekit join failed: %w", err)
	}

	s := newSession(peerID, h, logger, conn)
	s.onLost = func(err error) {
		a.forget(peerID, s)
		h.OnSignalingLost(err)
	}
	a.mu.Lock()
	if _, exists := a.sessions[peerID]; exists {
		a.mu.Unlock()
		s.close()
		return fmt.Errorf("peer %s is already connected", peerID)
	}
	a.sessions[peerID] = s
	a.mu.Unlock()
	go s.readLoop()

	if err := a.join(ctx, s); err != nil {
		if a.forget(peerID, s) {
			s.leave()
		}
		return fmt.Errorf("livekit join failed: %w", err)
	}
	s.ready.Store(true)
	// A loss between the last join step and ready took the pre-ready path:
	// it is still waiting in failed.
	select {
	case err := <-s.failed:
		if a.forget(peerID, s) {
			s.close()
		}
		return fmt.Errorf("livekit join failed: %w", err)
	default:
	}
	logger.Info("LiveKit adapter: peer joined")
	return nil
}

// join runs the join steps of data-model "Bot Session" up to ready.
func (a *Adapter) join(ctx context.Context, s *session) error {
	var j *joinResponse
	select {
	case j = <-s.joined:
	case err := <-s.failed:
		return err
	case <-ctx.Done():
		return fmt.Errorf("no JoinResponse: %w", ctx.Err())
	}
	if j.AlternativeURL != "" {
		return fmt.Errorf("server redirects to %s (multi-node is not supported)", j.AlternativeURL)
	}
	s.logger.Debug("LiveKit join response", slog.String("server_version", j.ServerVersion), slog.Int("server_protocol", int(j.ServerProtocol)),
		slog.String("participant_sid", j.Participant.SID), slog.Int("others", len(j.OtherParticipants)))
	go s.keepalive(s.pingEvery, s.pingTimeout)

	if len(s.h.LocalMediaKinds()) > 0 {
		if err := a.publish(ctx, s); err != nil {
			return err
		}
	}
	select {
	case <-s.subAnswered:
		return nil
	case err := <-s.failed:
		return err
	case <-ctx.Done():
		return fmt.Errorf("no subscriber offer: %w", ctx.Err())
	}
}

// localTrack is a section of the publish offer.
type localTrack struct {
	kind string // "audio" or "video"
	id   string // msid track ID: the AddTrackRequest cid
}

// offerTracks returns the audio and video sections of an offer with their
// msid track IDs.
func offerTracks(sdp string) []localTrack {
	var tracks []localTrack
	kind := ""
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "m="):
			kind = ""
			if m := strings.Fields(line[2:]); len(m) > 0 && (m[0] == "audio" || m[0] == "video") {
				kind = m[0]
			}
		case kind != "" && strings.HasPrefix(line, "a=msid:"):
			if f := strings.Fields(strings.TrimPrefix(line, "a=msid:")); len(f) == 2 {
				tracks = append(tracks, localTrack{kind: kind, id: f[1]})
				kind = "" // One track per section
			}
		}
	}
	return tracks
}

// addTrackFor is the AddTrackRequest of one published track. A video track
// in another codec than VP8 names it, as LiveKit's clients do (014 research
// R10); VP8 single stream sends 008's request. A layered track (heights
// low first) also announces its layers, as LiveKit's JavaScript client
// does: simulcast by the RIDs the bot sends, one per height; SVC as
// spatial layers of one stream, which the server reads from the
// dependency descriptor. LiveKit has no field for the scalability mode.
func addTrackFor(kind, cid string, width, height int, codec media.VideoCodec, arr media.Arrangement, heights []int) *addTrackRequest {
	req := &addTrackRequest{CID: cid, Name: kind}
	if kind == "audio" {
		req.Type, req.Source, req.DisableRed = trackAudio, sourceMicrophone, true
		return req
	}
	if codec == "" {
		codec = media.CodecVP8
	}
	req.Type, req.Source, req.Width, req.Height = trackVideo, sourceCamera, uint32(width), uint32(height)
	layered := (arr == media.ArrangementSimulcast || arr == media.ArrangementSVC) && len(heights) > 0
	if !layered {
		if codec != media.CodecVP8 {
			req.Codecs = []simulcastCodec{{Codec: string(codec), CID: cid}}
		}
		return req
	}
	top := heights[len(heights)-1]
	req.Width, req.Height = uint32(widthOf(top)), uint32(top)
	bitrates := media.ContentBitrates(codec, arr, top)
	mode := layerModeOnePerStream
	if arr == media.ArrangementSVC {
		mode = layerModeMultiplePerStream
	}
	for i, h := range heights {
		l := videoLayer{
			Quality: qualityHigh - int32(len(heights)-1-i),
			Width:   uint32(widthOf(h)), Height: uint32(h),
			Bitrate: uint32(bitrates[h] * 1000),
		}
		if l.Quality < qualityLow {
			l.Quality = qualityLow
		}
		if arr == media.ArrangementSimulcast && i < len(media.SimulcastRIDs) {
			l.RID = media.SimulcastRIDs[i]
		}
		req.Layers = append(req.Layers, l)
	}
	req.Codecs = []simulcastCodec{{Codec: string(codec), CID: cid, Layers: req.Layers, Mode: mode}}
	return req
}

// widthOf is the width of a 16:9 picture of height h.
func widthOf(h int) int { return h * 16 / 9 }

// trackSettings asks for the requested height on the given video tracks,
// or is nil when the bot takes the top layer (requested 0) or has none.
func trackSettings(videoSIDs []string, requested int) *updateTrackSettings {
	if requested <= 0 || len(videoSIDs) == 0 {
		return nil
	}
	return &updateTrackSettings{TrackSIDs: videoSIDs, Width: uint32(widthOf(requested)), Height: uint32(requested)}
}

// publish announces and publishes the bot's tracks (research R5).
func (a *Adapter) publish(ctx context.Context, s *session) error {
	offer, err := s.h.CreatePublishOffer(s.trickle(targetPublisher))
	if err != nil {
		return fmt.Errorf("publish offer: %w", err)
	}
	tracks := offerTracks(offer)
	if len(tracks) == 0 {
		return errors.New("publish offer has no track")
	}
	pending := map[string]bool{}
	for _, t := range tracks {
		w, h := s.h.LocalVideoSize()
		req := addTrackFor(t.kind, t.id, w, h, a.cfg.VideoCodec, a.cfg.Arrangement, s.h.VideoLayerHeights())
		if err := s.send(signalRequest{AddTrack: req}); err != nil {
			return fmt.Errorf("add track: %w", err)
		}
		pending[t.id] = true
	}
	for len(pending) > 0 {
		select {
		case p := <-s.published:
			delete(pending, p.CID)
		case err := <-s.failed:
			return err
		case <-ctx.Done():
			return fmt.Errorf("tracks not published: %w", ctx.Err())
		}
	}
	if err := s.send(signalRequest{Offer: &sessionDescription{Type: "offer", SDP: offer, ID: 1}}); err != nil {
		return fmt.Errorf("send publish offer: %w", err)
	}
	select {
	case ans := <-s.pubAnswer:
		if err := s.h.ApplyPublishAnswer(ans.SDP); err != nil {
			return fmt.Errorf("publish answer: %w", err)
		}
		return nil
	case err := <-s.failed:
		return err
	case <-ctx.Done():
		return fmt.Errorf("no publish answer: %w", ctx.Err())
	}
}

// forget removes s from the registry if it is still registered for
// peerID, and reports whether it did.
func (a *Adapter) forget(peerID string, s *session) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessions[peerID] != s {
		return false
	}
	delete(a.sessions, peerID)
	return true
}

// Disconnect leaves the room (LeaveRequest) and closes the connection. It
// is safe for unknown or already disconnected peers.
func (a *Adapter) Disconnect(peerID string) error {
	a.mu.Lock()
	s, ok := a.sessions[peerID]
	delete(a.sessions, peerID)
	a.mu.Unlock()
	if !ok {
		return nil
	}
	a.logger.Info("LiveKit adapter: disconnecting peer", slog.String("peer_id", peerID))
	s.leave()
	return nil
}
