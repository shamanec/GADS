/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

// annexBStartCode is the H.264 Annex-B start code sequence.
var annexBStartCode = []byte{0x00, 0x00, 0x00, 0x01}

func makeAnnexBFrame(units ...[]byte) []byte {
	var frame []byte
	for _, u := range units {
		frame = append(frame, annexBStartCode...)
		frame = append(frame, u...)
	}
	return frame
}

// makeWebSocketMessage builds the wire format sent by Android:
// [8 bytes big-endian PTS][payload]
func makeWebSocketMessage(pts int64, payload []byte) []byte {
	msg := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint64(msg[0:8], uint64(pts))
	copy(msg[8:], payload)
	return msg
}

// ---------------------------------------------------------------------------
// extractNALUnits
// ---------------------------------------------------------------------------

func TestExtractNALUnits_EmptyInput_ReturnsNil(t *testing.T) {
	got := extractNALUnits([]byte{})
	assert.Nil(t, got)
}

func TestExtractNALUnits_NoStartCode_ReturnsNil(t *testing.T) {
	got := extractNALUnits([]byte{0xAA, 0xBB, 0xCC, 0xDD})
	assert.Nil(t, got)
}

func TestExtractNALUnits_SingleNALUnit_ReturnsSingleEntry(t *testing.T) {
	payload := []byte{0x65, 0x10, 0x20} // IDR slice
	data := makeAnnexBFrame(payload)

	got := extractNALUnits(data)

	assert.Len(t, got, 1)
	assert.Equal(t, data, got[0])
}

func TestExtractNALUnits_MultipleNALUnits_ReturnsAllEntries(t *testing.T) {
	sps := []byte{0x67, 0x42, 0x00, 0x1e} // SPS NAL
	pps := []byte{0x68, 0xce, 0x38, 0x80} // PPS NAL
	idr := []byte{0x65, 0x10, 0x20, 0x30} // IDR slice

	data := makeAnnexBFrame(sps, pps, idr)

	got := extractNALUnits(data)

	assert.Len(t, got, 3)
	// Each extracted unit must start with the Annex-B start code.
	for _, unit := range got {
		assert.Equal(t, annexBStartCode, unit[:4])
	}
}

func TestExtractNALUnits_TwoNALUnits_SecondUnitStartsAtSecondStartCode(t *testing.T) {
	first := []byte{0x67, 0x01, 0x02}
	second := []byte{0x68, 0x03, 0x04}

	data := makeAnnexBFrame(first, second)

	got := extractNALUnits(data)

	assert.Len(t, got, 2)

	expectedFirst := append(annexBStartCode, first...)
	expectedSecond := append(annexBStartCode, second...)
	assert.Equal(t, expectedFirst, got[0])
	assert.Equal(t, expectedSecond, got[1])
}

// ---------------------------------------------------------------------------
// parseH264Message
// ---------------------------------------------------------------------------

func TestParseH264Message_ValidMessage_ReturnsPTSAndData(t *testing.T) {
	const wantPTS int64 = 123456789
	payload := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x10} // 6 bytes of H.264

	msg := makeWebSocketMessage(wantPTS, payload)

	pts, h264Data, err := parseH264Message(msg)

	assert.NoError(t, err)
	assert.Equal(t, wantPTS, pts)
	assert.Equal(t, payload, h264Data)
}

func TestParseH264Message_TooShort_ReturnsError(t *testing.T) {
	// Message must be at least 13 bytes (8 PTS + 5 H.264 minimum).
	msg := make([]byte, 12)

	_, _, err := parseH264Message(msg)

	assert.Error(t, err)
}

func TestParseH264Message_ExactlyMinimumLength_Succeeds(t *testing.T) {
	const wantPTS int64 = 42
	// 5 bytes of H.264 so total = 8 + 5 = 13.
	payload := []byte{0x00, 0x00, 0x00, 0x01, 0x65}

	msg := makeWebSocketMessage(wantPTS, payload)

	pts, h264Data, err := parseH264Message(msg)

	assert.NoError(t, err)
	assert.Equal(t, wantPTS, pts)
	assert.Equal(t, payload, h264Data)
}

// ---------------------------------------------------------------------------
// parsePCMMessage
// ---------------------------------------------------------------------------

func TestParsePCMMessage_ValidMessage_ReturnsPTSAndPCM(t *testing.T) {
	const wantPTS int64 = 987654321
	// 4 PCM int16 samples encoded as little-endian bytes
	pcm := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	msg := makeWebSocketMessage(wantPTS, pcm)

	pts, pcmData, err := parsePCMMessage(msg)

	assert.NoError(t, err)
	assert.Equal(t, wantPTS, pts)
	assert.Equal(t, pcm, pcmData)
}

func TestParsePCMMessage_TooShort_ReturnsError(t *testing.T) {
	// Message must have at least 8 bytes for the PTS.
	msg := make([]byte, 7)

	_, _, err := parsePCMMessage(msg)

	assert.Error(t, err)
}

func TestParsePCMMessage_Exactly8Bytes_ReturnsEmptyPCM(t *testing.T) {
	const wantPTS int64 = 0
	msg := make([]byte, 8) // only PTS, no PCM data

	pts, pcmData, err := parsePCMMessage(msg)

	assert.NoError(t, err)
	assert.Equal(t, wantPTS, pts)
	assert.Empty(t, pcmData)
}

// ---------------------------------------------------------------------------
// decodePCMToInt16
// ---------------------------------------------------------------------------

func TestDecodePCMToInt16_EmptyInput_ReturnsEmptySlice(t *testing.T) {
	got := decodePCMToInt16([]byte{})
	assert.Empty(t, got)
}

func TestDecodePCMToInt16_OddByteCount_IgnoresTrailingByte(t *testing.T) {
	// 3 bytes → only 1 complete int16 sample (first 2 bytes)
	got := decodePCMToInt16([]byte{0x01, 0x02, 0xFF})
	assert.Len(t, got, 1)
}

func TestDecodePCMToInt16_KnownValues_DecodesCorrectly(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []int16
	}{
		{
			name:  "zero samples",
			input: []byte{0x00, 0x00, 0x00, 0x00},
			want:  []int16{0, 0},
		},
		{
			name: "little-endian sample 0x0201",
			// low byte first: 0x01, high byte: 0x02 → int16(0x0201) = 513
			input: []byte{0x01, 0x02},
			want:  []int16{0x0201},
		},
		{
			name: "negative sample",
			// 0xFF, 0xFF → int16(-1) in little-endian
			input: []byte{0xFF, 0xFF},
			want:  []int16{-1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decodePCMToInt16(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// padOrTruncatePCM
// ---------------------------------------------------------------------------

func TestPadOrTruncatePCM_ShorterThanTarget_PadsWithZeros(t *testing.T) {
	input := []int16{1, 2, 3}
	got := padOrTruncatePCM(input, 5)

	assert.Len(t, got, 5)
	assert.Equal(t, []int16{1, 2, 3, 0, 0}, got)
}

func TestPadOrTruncatePCM_LongerThanTarget_Truncates(t *testing.T) {
	input := []int16{1, 2, 3, 4, 5}
	got := padOrTruncatePCM(input, 3)

	assert.Equal(t, []int16{1, 2, 3}, got)
}

func TestPadOrTruncatePCM_ExactSize_ReturnsSameContent(t *testing.T) {
	input := []int16{10, 20, 30}
	got := padOrTruncatePCM(input, 3)

	assert.Equal(t, input, got)
}
