package media

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// ContentInfo describes one embedded video content file: its codec, how it
// is layered, the height of one of its layers, and what that layer costs
// at the picture quality of the VP8 content (specs/014 research R3). An
// SVC file has one entry per layer, all with its MaxHeight.
type ContentInfo struct {
	Codec       VideoCodec  `json:"codec"`
	Arrangement Arrangement `json:"arrangement"` // single (also the simulcast layers) or svc
	MaxHeight   int         `json:"max_height,omitempty"`
	Height      int         `json:"height"`
	BitrateKbps int         `json:"bitrate_kbps"`
	// VMAF of this layer against the source at its size, and the VMAF of
	// the VP8 content of the same height, which it must be within 1.0 of.
	VMAF          float64 `json:"vmaf"`
	VMAFReference float64 `json:"vmaf_reference"`
	File          string  `json:"file"`
}

type manifest []ContentInfo

// Written by scripts/gen-media.sh with the files it lists.
//
//go:embed assets/content.json
var manifestJSON []byte

var (
	manifestOnce sync.Once
	manifestVal  manifest
	manifestErr  error
)

func loadManifest() (manifest, error) {
	manifestOnce.Do(func() {
		if err := json.Unmarshal(manifestJSON, &manifestVal); err != nil {
			manifestErr = fmt.Errorf("content.json: %w", err)
		}
	})
	return manifestVal, manifestErr
}

// lookup returns the entry of one layer. Simulcast layers are single-stream
// files; an SVC layer is looked up by its file's maximum height.
func (m manifest) lookup(c VideoCodec, arr Arrangement, height int, maxHeight ...int) (ContentInfo, error) {
	if arr == ArrangementSimulcast {
		arr = ArrangementSingle
	}
	mh := 0
	if arr == ArrangementSVC && len(maxHeight) > 0 {
		mh = maxHeight[0]
	}
	for _, e := range m {
		if e.Codec == c && e.Arrangement == arr && e.Height == height && (mh == 0 || e.MaxHeight == mh) {
			return e, nil
		}
	}
	return ContentInfo{}, fmt.Errorf("no %s %s content of %d lines in content.json", c, arr, height)
}

// heightsFor returns the heights a run sends, low to high.
func heightsFor(arr Arrangement, maxHeight int) ([]int, error) {
	if arr == ArrangementSingle {
		return []int{defaultHeight}, nil
	}
	return LayerHeights(maxHeight)
}

// defaultHeight is the single stream's height (004 content).
const defaultHeight = 360

// ContentBitrates returns, by height, the bitrate in kbit/s of the content
// a run sends; nil when content.json lacks one of them.
func ContentBitrates(c VideoCodec, arr Arrangement, maxHeight int) map[int]int {
	m, err := loadManifest()
	if err != nil {
		return nil
	}
	heights, err := heightsFor(arr, maxHeight)
	if err != nil {
		return nil
	}
	out := map[int]int{}
	for _, h := range heights {
		e, err := m.lookup(c, arr, h, maxHeight)
		if err != nil {
			return nil
		}
		out[h] = e.BitrateKbps
	}
	return out
}
