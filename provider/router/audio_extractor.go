/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

// PCMAudioExtractor consumes raw 48kHz/mono/16-bit-LE PCM and encodes it to Opus for WebRTC.
// Platform-specific connection setup lives in audio_extractor_{android,ios}.go.

import (
	"GADS/common/models"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/hraban/opus"
)

// AudioFrame represents an audio frame with its presentation timestamp
type AudioFrame struct {
	Data []byte // Opus-encoded audio data
	PTS  int64  // Presentation timestamp in microseconds
}

// PCMAudioExtractor pulls PCM from a producer stream and encodes it to Opus. The producer
// envelope is platform-specific; the Opus path is shared.
type PCMAudioExtractor struct {
	device       *models.DBDevice
	conn         io.ReadWriteCloser
	audioChannel chan AudioFrame
	ctx          context.Context
	cancel       context.CancelFunc
	encoder      *opus.Encoder
}

// newOpusEncoder builds the shared Opus encoder configuration used by every platform:
// 48kHz mono, 64kbps, in-band FEC, fullband.
func newOpusEncoder() (*opus.Encoder, error) {
	encoder, err := opus.NewEncoder(48000, 1, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("failed to create Opus encoder: %w", err)
	}
	if err := encoder.SetBitrate(64000); err != nil {
		return nil, fmt.Errorf("failed to set opus bitrate: %w", err)
	}
	if err := encoder.SetInBandFEC(true); err != nil {
		return nil, fmt.Errorf("failed to enable opus fec: %w", err)
	}
	if err := encoder.SetPacketLossPerc(5); err != nil {
		return nil, fmt.Errorf("failed to set opus packet loss perc: %w", err)
	}
	if err := encoder.SetMaxBandwidth(opus.Fullband); err != nil {
		return nil, fmt.Errorf("failed to set opus max bandwidth: %w", err)
	}
	return encoder, nil
}

// parsePCMMessage parses [8B big-endian PTS][PCM data], returning the PTS and raw PCM
// (error if the message is shorter than 8 bytes).
func parsePCMMessage(msg []byte) (pts int64, pcmData []byte, err error) {
	if len(msg) < 8 {
		return 0, nil, fmt.Errorf("message too short: got %d bytes, need at least 8", len(msg))
	}
	pts = int64(binary.BigEndian.Uint64(msg[0:8]))
	pcmData = msg[8:]
	return pts, pcmData, nil
}

// decodePCMToInt16 converts raw PCM bytes (little-endian int16 pairs) to []int16.
// Trailing odd bytes are ignored.
func decodePCMToInt16(pcmData []byte) []int16 {
	numSamples := len(pcmData) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
	}
	return samples
}

// padOrTruncatePCM returns a slice of exactly size samples from input.
// Missing samples are filled with zeros; excess samples are discarded.
func padOrTruncatePCM(samples []int16, size int) []int16 {
	out := make([]int16, size)
	copy(out, samples)
	return out
}

// waitForAudioPort waits up to maxWait for the audio TCP port to accept connections.
func waitForAudioPort(ctx context.Context, address string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("audio port %s not ready after %v", address, maxWait)
}

// GetAudioChannel returns the channel for reading audio frames
func (e *PCMAudioExtractor) GetAudioChannel() <-chan AudioFrame {
	return e.audioChannel
}

// Close stops the extractor
func (e *PCMAudioExtractor) Close() {
	e.cancel()
	if e.conn != nil {
		e.conn.Close()
	}
}
