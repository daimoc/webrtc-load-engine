# Golden LiveKit signaling frames

Marshalled with `github.com/livekit/protocol@v1.52.1` (`proto.MarshalOptions{Deterministic: true}`) by a throwaway generator (008 T002, not committed). `req_*` are `SignalRequest`, `resp_*` are `SignalResponse`.

## req_add_track_audio.bin (29 bytes)

```text
add_track:  {
  cid:  "audio-track-id"
  name:  "audio"
  source:  MICROPHONE
  disable_red:  true
}

```

## req_add_track_video.bin (35 bytes)

```text
add_track:  {
  cid:  "video-track-id"
  name:  "video"
  type:  VIDEO
  width:  640
  height:  360
  source:  CAMERA
}

```

## req_answer.bin (27 bytes)

```text
answer:  {
  type:  "answer"
  sdp:  "v=0\r\nanswer\r\n"
  id:  7
}

```

## req_leave.bin (4 bytes)

```text
leave:  {
  reason:  CLIENT_INITIATED
}

```

## req_offer.bin (44 bytes)

```text
offer:  {
  type:  "offer"
  sdp:  "v=0\r\no=- 1 2 IN IP4 127.0.0.1\r\n"
  id:  1
}

```

## req_ping.bin (12 bytes)

```text
ping_req:  {
  timestamp:  1760000000000
  rtt:  12
}

```

## req_subscription.bin (18 bytes)

```text
subscription:  {
  track_sids:  "TR_a2"
  track_sids:  "TR_v2"
  subscribe:  true
}

```

## req_trickle_pub.bin (107 bytes)

```text
trickle:  {
  candidateInit:  "{\"candidate\":\"candidate:1 1 udp 2130706431 192.168.1.65 50000 typ host\",\"sdpMid\":\"0\",\"sdpMLineIndex\":0}"
}

```

## req_trickle_sub.bin (109 bytes)

```text
trickle:  {
  candidateInit:  "{\"candidate\":\"candidate:1 1 udp 2130706431 192.168.1.65 50000 typ host\",\"sdpMid\":\"0\",\"sdpMLineIndex\":0}"
  target:  SUBSCRIBER
}

```

## resp_answer.bin (35 bytes)

```text
answer:  {
  type:  "answer"
  sdp:  "v=0\r\npublish answer\r\n"
  id:  1
}

```

## resp_join.bin (252 bytes)

```text
join:  {
  room:  {
    sid:  "RM_x"
    name:  "load008"
  }
  participant:  {
    sid:  "PA_self"
    identity:  "bot-1"
    state:  JOINED
  }
  other_participants:  {
    sid:  "PA_2"
    identity:  "bot-2"
    state:  ACTIVE
    tracks:  {
      sid:  "TR_a2"
      name:  "audio"
      source:  MICROPHONE
    }
    tracks:  {
      sid:  "TR_v2"
      type:  VIDEO
      name:  "video"
      width:  640
      height:  360
      source:  CAMERA
      mime_type:  "video/VP8"
    }
    is_publisher:  true
  }
  other_participants:  {
    sid:  "PA_3"
    identity:  "bot-3"
    state:  ACTIVE
    tracks:  {
      sid:  "TR_a3"
      name:  "audio"
      source:  MICROPHONE
    }
    tracks:  {
      sid:  "TR_v3"
      type:  VIDEO
      name:  "video"
      width:  640
      height:  360
      source:  CAMERA
      mime_type:  "video/VP8"
    }
    is_publisher:  true
  }
  server_version:  "1.13.10"
  ice_servers:  {
    urls:  "stun:stun.example.org:3478"
  }
  subscriber_primary:  true
  ping_timeout:  15
  ping_interval:  5
  server_info:  {
    version:  "1.13.10"
    protocol:  16
    node_id:  "ND_x"
  }
}

```

## resp_join_alternative.bin (27 bytes)

```text
join:  {
  alternative_url:  "wss://other.example.org"
}

```

## resp_leave.bin (8 bytes)

```text
leave:  {
  can_reconnect:  true
  reason:  SERVER_SHUTDOWN
  action:  RECONNECT
}

```

## resp_offer.bin (47 bytes)

```text
offer:  {
  type:  "offer"
  sdp:  "v=0\r\nsubscribe offer\r\n"
  id:  3
  mid_to_track_id:  {
    key:  "0"
    value:  "TR_a2"
  }
}

```

## resp_pong.bin (8 bytes)

```text
pong:  1760000000000

```

## resp_pong_resp.bin (17 bytes)

```text
pong_resp:  {
  last_ping_timestamp:  1760000000000
  timestamp:  1760000000020
}

```

## resp_refresh_token.bin (20 bytes)

```text
refresh_token:  "eyJ.refresh.token"

```

## resp_speakers_changed.bin (17 bytes)

```text
speakers_changed:  {
  speakers:  {
    sid:  "PA_2"
    level:  0.5
    active:  true
  }
}

```

## resp_subscription_response.bin (12 bytes)

```text
subscription_response:  {
  track_sid:  "TR_v3"
  err:  SE_TRACK_NOTFOUND
}

```

## resp_track_published.bin (36 bytes)

```text
track_published:  {
  cid:  "video-track-id"
  track:  {
    sid:  "TR_vself"
    type:  VIDEO
    muted:  true
    source:  CAMERA
  }
}

```

## resp_track_unpublished.bin (13 bytes)

```text
track_unpublished:  {
  track_sid:  "TR_vself"
}

```

## resp_trickle.bin (109 bytes)

```text
trickle:  {
  candidateInit:  "{\"candidate\":\"candidate:1 1 udp 2130706431 192.168.1.65 50000 typ host\",\"sdpMid\":\"0\",\"sdpMLineIndex\":0}"
  target:  SUBSCRIBER
}

```

## resp_trickle_pub.bin (107 bytes)

```text
trickle:  {
  candidateInit:  "{\"candidate\":\"candidate:1 1 udp 2130706431 192.168.1.65 50000 typ host\",\"sdpMid\":\"0\",\"sdpMLineIndex\":0}"
}

```

## resp_update.bin (73 bytes)

```text
update:  {
  participants:  {
    sid:  "PA_2"
    identity:  "bot-2"
    state:  DISCONNECTED
  }
  participants:  {
    sid:  "PA_4"
    identity:  "bot-4"
    state:  ACTIVE
    tracks:  {
      sid:  "TR_v4"
      type:  VIDEO
      name:  "video"
      width:  640
      height:  360
      source:  CAMERA
      mime_type:  "video/VP8"
    }
  }
}

```

