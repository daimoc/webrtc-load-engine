package janus

import (
	"errors"
	"fmt"
	"slices"

	"github.com/pion/sdp/v3"

	"webrtc-load-engine/src/media"
)

// publishJSEP is a bot's publish offer with what Janus needs to know of its
// layers (014 T054). Simulcast: the Janus core reads the RIDs in the order
// of the offer's rid lines and takes the first as the highest unless told
// "lmh"; Pion lists them low first (janus.c rid_order, sdp.c). SVC: the
// video section's index, MID and scalability mode, or the VideoRoom treats
// it as a single stream (janus.c checks each entry's "svc" string).
func publishJSEP(offer string, arr media.Arrangement) (*JSEP, error) {
	j := &JSEP{Type: "offer", SDP: offer}
	switch arr {
	case media.ArrangementSimulcast:
		j.RIDOrder = "lmh"
	case media.ArrangementSVC:
		var d sdp.SessionDescription
		if err := d.Unmarshal([]byte(offer)); err != nil {
			return nil, fmt.Errorf("publish offer: %w", err)
		}
		i := slices.IndexFunc(d.MediaDescriptions, func(m *sdp.MediaDescription) bool { return m.MediaName.Media == "video" })
		if i < 0 {
			return nil, errors.New("publish offer: no video section")
		}
		mid, _ := d.MediaDescriptions[i].Attribute("mid")
		j.SVC = []JSEPSVC{{MIndex: i, MID: mid, Mode: "L3T3_KEY"}}
	}
	return j, nil
}

// streamLayer is one stream of a configure request.
type streamLayer struct {
	MID           string `json:"mid"`
	Substream     *int   `json:"substream,omitempty"`
	SpatialLayer  *int   `json:"spatial_layer,omitempty"`
	TemporalLayer *int   `json:"temporal_layer,omitempty"`
}

// layerConfigure is the subscriber's configure request asking for the
// requested height on every received video stream but the dummy's:
// substream for simulcast, spatial_layer with every temporal layer for
// SVC. requested 0 means the top layer. Nil without layers or video.
func layerConfigure(streams []SubStream, dummy uint64, arr media.Arrangement, heights []int, requested int) map[string]any {
	if (arr != media.ArrangementSimulcast && arr != media.ArrangementSVC) || len(heights) == 0 {
		return nil
	}
	layer := len(heights) - 1
	if requested > 0 {
		layer = 0
		for i, h := range heights {
			if h <= requested {
				layer = i
			}
		}
	}
	var out []streamLayer
	for _, st := range streams {
		if st.Type != "video" || st.FeedID == dummy {
			continue
		}
		l := streamLayer{MID: st.MID}
		n := layer
		if arr == media.ArrangementSimulcast {
			l.Substream = &n
		} else {
			all := 2
			l.SpatialLayer, l.TemporalLayer = &n, &all
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return nil
	}
	return map[string]any{"request": "configure", "streams": out}
}
