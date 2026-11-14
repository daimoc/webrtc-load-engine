package execution

import (
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/scenario"
)

const (
	// worstBots bounds each ranking of the quality summary (FR-015).
	worstBots = 10
	// freezeLogInterval bounds the freeze logs per stream (FR-017).
	freezeLogInterval = time.Minute
	// mouthToEarExtra is added to the network delay for the score: the
	// playout delay (60 ms) and one audio frame (20 ms) (research R4).
	mouthToEarExtra = 80 * time.Millisecond
)

// botQuality is the per-bot quality record (data-model "Per-Bot Quality
// Record"), kept until the scenario ends. Counters are the sums of the
// bot's streams' cumulative values at the last collection.
type botQuality struct {
	video, audio bool // Received at least one frame of the kind
	requested    int  // Video height the bot asks for

	rateSum float64 // Frame rate samples, one per video stream per period
	rateN   int

	freezes    uint64
	freezeTime time.Duration
	heightTime map[int]time.Duration
	switches   uint64
	kfSent     uint64
	kfReceived uint64

	concealed time.Duration
	expected  time.Duration
	mosSum    float64 // Score samples, one per audio stream per period
	mosN      int

	roundTrip time.Duration // Median of the last round-trip samples of the bot's own streams
	freezeLog []streamFreezeLog
	heights   []int // Last height logged per received video stream (015)

	latency map[string]*media.LatencyBuckets // By kind, over the bot's streams
	lookups map[string]uint64                // By kind: frames looked up in the send times, found or not
}

// streamFreezeLog rate-limits the freeze logs of one received video stream.
type streamFreezeLog struct {
	freezes uint64
	logged  time.Time
}

func (c *mediaCollector) bot(peerID string) *botQuality {
	b := c.bots[peerID]
	if b == nil {
		b = &botQuality{heightTime: map[int]time.Duration{}, latency: map[string]*media.LatencyBuckets{}, lookups: map[string]uint64{}}
		c.bots[peerID] = b
	}
	return b
}

// collectQuality exports the quality measures of a peer's snapshot and
// updates its record.
func (c *mediaCollector) collectQuality(peerID string, requested int, snap map[string]KindMediaSnapshot, now time.Time) {
	var rtts []time.Duration
	for _, kind := range mediaKinds {
		rtts = append(rtts, snap[kind].RoundTrips...)
	}
	video, audio := snap[media.KindVideo], snap[media.KindAudio]
	if len(video.Video) == 0 && len(audio.Audio) == 0 && len(rtts) == 0 &&
		video.KeyframeRequestsReceived == 0 && c.bots[peerID] == nil {
		return // Nothing received yet: no record
	}
	b := c.bot(peerID)
	b.requested = requested // Before the video is collected: its height log names it
	if len(rtts) > 0 {
		sort.Slice(rtts, func(i, j int) bool { return rtts[i] < rtts[j] })
		b.roundTrip = rtts[len(rtts)/2]
	}
	c.collectVideo(peerID, b, video, now)
	c.collectAudio(b, audio)
}

func (c *mediaCollector) collectVideo(peerID string, b *botQuality, k KindMediaSnapshot, now time.Time) {
	id := c.scenarioID
	addCount(media.KeyframeRequestsSent.WithLabelValues(id), &b.kfSent, k.KeyframeRequestsSent)
	addCount(media.KeyframeRequestsReceived.WithLabelValues(id), &b.kfReceived, k.KeyframeRequestsReceived)
	if len(k.Video) == 0 {
		return
	}
	b.video = true
	var freezes, switches uint64
	var freezeTime time.Duration
	heightTime := map[int]time.Duration{}
	var latency media.LatencyBuckets
	var lookups uint64
	for i, s := range k.Video {
		latency.Merge(&s.Latency)
		lookups += s.LatencyLookups
		freezes += s.Freezes
		switches += s.Switches
		freezeTime += s.FreezeTime
		for h, d := range s.HeightTime {
			heightTime[h] += d
		}
		if s.PeriodIntervals > 0 {
			rate := s.FrameRate()
			media.FrameRate.WithLabelValues(id).Observe(rate)
			b.rateSum += rate
			b.rateN++
			if s.Height > 0 {
				media.FrameHeight.WithLabelValues(id).Observe(float64(s.Height))
			}
		}
		c.logFreeze(peerID, b, i, s, now)
		c.logHeight(peerID, b, i, s.Height)
	}
	if latency.N > 0 {
		b.latency[media.KindVideo] = &latency
	}
	b.lookups[media.KindVideo] = lookups
	addCount(media.Freezes.WithLabelValues(id), &b.freezes, freezes)
	addCount(media.ResolutionSwitches.WithLabelValues(id), &b.switches, switches)
	if freezeTime > b.freezeTime {
		media.FreezeSeconds.WithLabelValues(id).Add((freezeTime - b.freezeTime).Seconds())
		b.freezeTime = freezeTime
	}
	for h, d := range heightTime {
		if d > b.heightTime[h] {
			media.HeightSeconds.WithLabelValues(id, strconv.Itoa(h)).Add((d - b.heightTime[h]).Seconds())
			b.heightTime[h] = d
		}
	}
}

// logHeight logs each change of the height stream i is received at: the
// mediasoup comparison of 015 reads the layer forwarded over time from it,
// that server giving no per-receiver estimate (research R7).
func (c *mediaCollector) logHeight(peerID string, b *botQuality, i, height int) {
	for len(b.heights) <= i {
		b.heights = append(b.heights, 0)
	}
	if height == 0 || height == b.heights[i] {
		return
	}
	c.logger.Info("Received video height changed",
		slog.String("scenario_id", c.scenarioID), slog.String("peer_id", peerID), slog.Int("stream", i),
		slog.Int("previous_height", b.heights[i]), slog.Int("height", height), slog.Int("requested_height", b.requested))
	b.heights[i] = height
}

// logFreeze logs new freezes of stream i, at most once per stream per
// freezeLogInterval.
func (c *mediaCollector) logFreeze(peerID string, b *botQuality, i int, s media.VideoQualitySnapshot, now time.Time) {
	for len(b.freezeLog) <= i {
		b.freezeLog = append(b.freezeLog, streamFreezeLog{})
	}
	l := &b.freezeLog[i]
	if s.Freezes <= l.freezes {
		return
	}
	newFreezes := s.Freezes - l.freezes
	l.freezes = s.Freezes
	if !l.logged.IsZero() && now.Sub(l.logged) < freezeLogInterval {
		return
	}
	l.logged = now
	c.logger.Warn("Video freeze",
		slog.String("scenario_id", c.scenarioID), slog.String("peer_id", peerID),
		slog.Int("stream", i), slog.Uint64("new_freezes", newFreezes), slog.Duration("total_freeze_time", s.FreezeTime))
}

func (c *mediaCollector) collectAudio(b *botQuality, k KindMediaSnapshot) {
	if len(k.Audio) == 0 {
		return
	}
	b.audio = true
	id := c.scenarioID
	var concealed, expected time.Duration
	var latency media.LatencyBuckets
	var lookups uint64
	for _, s := range k.Audio {
		latency.Merge(&s.Latency)
		lookups += s.LatencyLookups
		concealed += time.Duration(s.Concealed) * s.Frame
		expected += time.Duration(s.Expected) * s.Frame
		if s.PeriodExpected == 0 {
			continue
		}
		// The stream's own latency when known, else half the round trip
		// (research R4).
		delay := b.roundTrip / 2
		if median, ok := s.Latency.Quantile(0.5); ok {
			delay = median
		}
		lossPct := 100 * float64(s.PeriodConcealed) / float64(s.PeriodExpected)
		mos := media.MOS(lossPct, delay+mouthToEarExtra)
		media.AudioMOS.WithLabelValues(id).Observe(mos)
		b.mosSum += mos
		b.mosN++
	}
	if latency.N > 0 {
		b.latency[media.KindAudio] = &latency
	}
	b.lookups[media.KindAudio] = lookups
	if concealed > b.concealed {
		media.AudioConcealedSeconds.WithLabelValues(id).Add((concealed - b.concealed).Seconds())
		b.concealed = concealed
	}
	if expected > b.expected {
		media.AudioExpectedSeconds.WithLabelValues(id).Add((expected - b.expected).Seconds())
		b.expected = expected
	}
}

// addCount adds to counter the growth of a cumulative value over *last, and
// stores it.
func addCount(counter interface{ Add(float64) }, last *uint64, value uint64) {
	if value > *last {
		counter.Add(float64(value - *last))
		*last = value
	}
}

// summary builds the quality summary from the per-bot records (FR-015).
func (c *mediaCollector) summary(now time.Time) *scenario.QualitySummary {
	s := &scenario.QualitySummary{
		ScenarioID:  c.scenarioID,
		GeneratedAt: now,
		Bots:        len(c.bots),
		Worst: scenario.WorstBots{
			LowestMOS:                []scenario.BotValue{},
			MostFreezeSeconds:        []scenario.BotValue{},
			HighestLatencyP95Seconds: []scenario.BotValue{},
		},
	}
	var rates, freezeSecs, ratios, scores []float64
	var mosBots, freezeBots, latencyBots, requestedBots []scenario.BotValue
	var requestedShares []float64
	latencies := map[string][]float64{} // By kind: each bot's median, seconds
	heightTime := map[int]time.Duration{}
	var totalHeight time.Duration
	for peerID, b := range c.bots {
		worstP95 := -1.0
		for kind, l := range b.latency {
			median, _ := l.Quantile(0.5)
			latencies[kind] = append(latencies[kind], median.Seconds())
			if p95, _ := l.Quantile(0.95); p95.Seconds() > worstP95 {
				worstP95 = p95.Seconds()
			}
		}
		if worstP95 >= 0 {
			latencyBots = append(latencyBots, scenario.BotValue{PeerID: peerID, Value: worstP95})
		}
		if b.video {
			s.Video.Bots++
			if b.rateN > 0 {
				rates = append(rates, b.rateSum/float64(b.rateN))
			}
			freezeSecs = append(freezeSecs, b.freezeTime.Seconds())
			if b.freezeTime > 0 {
				freezeBots = append(freezeBots, scenario.BotValue{PeerID: peerID, Value: b.freezeTime.Seconds()})
			}
			s.Video.FreezesTotal += int64(b.freezes)
			s.Video.ResolutionSwitchesTotal += int64(b.switches)
			var botTotal time.Duration
			for h, d := range b.heightTime {
				heightTime[h] += d
				totalHeight += d
				botTotal += d
			}
			if botTotal > 0 && b.requested > 0 {
				share := b.heightTime[b.requested].Seconds() / botTotal.Seconds()
				requestedShares = append(requestedShares, share)
				requestedBots = append(requestedBots, scenario.BotValue{PeerID: peerID, Value: share})
			}
		}
		s.Video.KeyframeRequestsSentTotal += int64(b.kfSent)
		s.Video.KeyframeRequestsReceivedTotal += int64(b.kfReceived)
		if b.audio {
			s.Audio.Bots++
			if b.expected > 0 {
				ratios = append(ratios, b.concealed.Seconds()/b.expected.Seconds())
			}
			if b.mosN > 0 {
				mos := b.mosSum / float64(b.mosN)
				scores = append(scores, mos)
				mosBots = append(mosBots, scenario.BotValue{PeerID: peerID, Value: mos})
			}
		}
	}

	if s.Video.Bots > 0 {
		s.Video.Available = true
		s.Video.FrameRate = percentiles(rates)
		s.Video.FreezeSeconds = percentiles(freezeSecs)
		s.Video.RequestedHeightShare = percentiles(requestedShares)
		if totalHeight > 0 {
			s.Video.HeightShare = map[string]float64{}
			for h, d := range heightTime {
				s.Video.HeightShare[strconv.Itoa(h)] = d.Seconds() / totalHeight.Seconds()
			}
		}
	} else {
		s.Video.Reason = "no bot received video"
	}
	if s.Audio.Bots > 0 {
		s.Audio.Available = true
		s.Audio.ConcealedRatio = percentiles(ratios)
		s.Audio.MOS = percentiles(scores)
	} else {
		s.Audio.Reason = "no bot received audio"
	}
	if l := latencies[media.KindVideo]; len(l) > 0 {
		s.Video.Latency, s.Video.LatencyAvailable = percentiles(l), true
	}
	if l := latencies[media.KindAudio]; len(l) > 0 {
		s.Audio.Latency, s.Audio.LatencyAvailable = percentiles(l), true
	}
	for kind, kq := range map[string]*scenario.KindQuality{media.KindVideo: &s.Video.KindQuality, media.KindAudio: &s.Audio.KindQuality} {
		if !kq.Available || kq.LatencyAvailable {
			continue
		}
		kq.LatencyReason = scenario.LatencyNotMeasured
		for _, b := range c.bots {
			if b.lookups[kind] > 0 {
				kq.LatencyReason = scenario.LatencyFrameNotIdentified
				break
			}
		}
	}
	s.Worst.HighestLatencyP95Seconds = worst(latencyBots, true)
	s.Worst.LowestMOS = worst(mosBots, false)
	s.Worst.LowestRequestedHeight = worst(requestedBots, false)
	s.Worst.MostFreezeSeconds = worst(freezeBots, true)
	s.ReceptionReports = c.receptionReports(s.Video.Bots)
	return s
}

// percentiles returns the nearest-rank percentiles of values, or nil for
// none.
func percentiles(values []float64) *scenario.Percentiles {
	if len(values) == 0 {
		return nil
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	rank := func(p float64) float64 {
		i := int(math.Ceil(p*float64(len(v)))) - 1
		if i < 0 {
			i = 0
		}
		return v[i]
	}
	return &scenario.Percentiles{P50: rank(0.50), P95: rank(0.95), P99: rank(0.99)}
}

// worst returns up to worstBots bots, worst first: highest values when
// highFirst, lowest otherwise; ties by peer ID.
func worst(bots []scenario.BotValue, highFirst bool) []scenario.BotValue {
	sort.Slice(bots, func(i, j int) bool {
		if bots[i].Value != bots[j].Value {
			return (bots[i].Value > bots[j].Value) == highFirst
		}
		return bots[i].PeerID < bots[j].PeerID
	})
	if len(bots) > worstBots {
		bots = bots[:worstBots]
	}
	if bots == nil {
		return []scenario.BotValue{}
	}
	return bots
}

// receptionReports builds the reception reports part of the summary (015
// research R6) from the peers' last snapshots. videoBots is the number of
// bots that received video.
func (c *mediaCollector) receptionReports(videoBots int) scenario.ReceptionReports {
	if videoBots == 0 {
		return scenario.ReceptionReports{Reason: scenario.ReceptionNoVideo}
	}
	var negotiated, counted bool
	var ids []string
	for id, r := range c.reception {
		negotiated = negotiated || r.negotiated
		counted = counted || (r.negotiated && r.counting)
		if r.negotiated && r.counting && r.arrived > 0 {
			ids = append(ids, id)
		}
	}
	switch {
	case !negotiated:
		return scenario.ReceptionReports{Reason: scenario.ReceptionNotNegotiated}
	case !counted || len(ids) == 0:
		return scenario.ReceptionReports{Reason: scenario.ReceptionNotCounted}
	}
	sort.Slice(ids, func(i, j int) bool { return launchOrderLess(ids[i], ids[j]) })
	out := scenario.ReceptionReports{
		Available: true,
		Bots:      len(ids),
		Limits:    scenario.ReceptionLimits{AcknowledgedShareMin: ackShareMin, LateShareMax: lateShareMax, LateIntervalMs: int(lateInterval / time.Millisecond)},
		Flagged:   []string{},
	}
	var shares, lates, lags []float64
	for _, id := range ids {
		r := c.reception[id]
		b := scenario.BotReception{
			PeerID:              id,
			PacketsArrived:      r.arrived,
			PacketsAcknowledged: r.acknowledged,
			AcknowledgedShare:   math.Min(1, float64(r.acknowledged)/float64(r.arrived)),
			FeedbacksSent:       r.feedbacks,
			IntervalMs:          scenario.MeanMax{Max: ms(r.intervalMax)},
			StampLagMs:          scenario.MeanMax{Max: ms(r.lagMax)},
		}
		if r.intervals > 0 {
			b.IntervalMs.Mean = ms(r.intervalSum) / float64(r.intervals)
			b.LateShare = float64(r.late) / float64(r.intervals)
		}
		if r.lagN > 0 {
			b.StampLagMs.Mean = ms(r.lagSum) / float64(r.lagN)
		}
		if b.AcknowledgedShare < ackShareMin || b.LateShare > lateShareMax {
			out.Flagged = append(out.Flagged, id)
		}
		shares = append(shares, b.AcknowledgedShare)
		lates = append(lates, b.LateShare)
		lags = append(lags, b.StampLagMs.Max)
		out.PerBot = append(out.PerBot, b)
	}
	out.AcknowledgedShare, out.LateShare, out.StampLagMaxMs = percentiles(shares), percentiles(lates), percentiles(lags)
	return out
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// launchOrderLess orders peer IDs ("peer-<scenario>-<i>") by their launch
// index, then as strings.
func launchOrderLess(a, b string) bool {
	ia, erra := strconv.Atoi(a[strings.LastIndexByte(a, '-')+1:])
	ib, errb := strconv.Atoi(b[strings.LastIndexByte(b, '-')+1:])
	if erra == nil && errb == nil && ia != ib {
		return ia < ib
	}
	return a < b
}
