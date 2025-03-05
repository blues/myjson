// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/blues/codec2/go"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Audio handler
func inboundWebAudioHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Handle file download
	if httpReq.Method == http.MethodGet {
		const prefix = "/audio/"
		if !strings.HasPrefix(httpReq.URL.Path, prefix) {
			http.Error(httpRsp, "Invalid URL", http.StatusBadRequest)
			return
		}
		filename := httpReq.URL.Path[len(prefix):]
		fullPath := filepath.Join(configDataDirectory+"audio/", filename)
		file, err := os.Open(fullPath)
		if err != nil {
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err == nil {
			httpRsp.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		} else {
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		httpRsp.Header().Set("Content-Type", "application/octet-stream")
		httpRsp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		httpRsp.WriteHeader(http.StatusOK)

		_, err = io.Copy(httpRsp, file)
		if err != nil {
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}

		return
	}

	// Get the payload if supplied
	pcmData, _ := io.ReadAll(httpReq.Body)

	// Extract key parameters
	header := "X-ResponseDevice"
	responseDeviceUID := httpReq.Header.Get(header)
	if responseDeviceUID == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	header = "X-ResponseProduct"
	responseProductUID := httpReq.Header.Get(header)
	if responseProductUID == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	header = "X-ResponseNotefile"
	responseNotefileID := httpReq.Header.Get(header)
	if responseNotefileID == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	header = "X-ResponseToken"
	responseApiToken := httpReq.Header.Get(header)
	if responseApiToken == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}

	// Remove the 1 byte of padding added by the notecard to "complete" the last page
	if len(pcmData) != 0 {
		pcmData = pcmData[:len(pcmData)-1]
	}

	// Exit if no audio, which is what's sent to synchronize when we are beginning a new transaction
	if len(pcmData) == 0 {
		httpRsp.WriteHeader(http.StatusOK)
		return
	}

	// Get the content type, which defines the audio format
	var rate int
	var c2data []byte
	contentType := httpReq.Header.Get("Content-Type")
	mediatype, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		errmsg := fmt.Sprintf("can't parse media type: %s", mediatype)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	if mediatype == "audio/l16" {
		rateStr := params["rate"]
		rate, err = strconv.Atoi(rateStr)
		if err != nil {
			errmsg := fmt.Sprintf("can't parse rate: %s", err)
			fmt.Printf("audio: %s\n", errmsg)
			httpRsp.WriteHeader(http.StatusOK)
			httpRsp.Write([]byte(errmsg))
			return
		}
	} else if mediatype == "audio/codec2-2400" {
		rateStr := params["rate"]
		rate, err = strconv.Atoi(rateStr)
		if err != nil {
			errmsg := fmt.Sprintf("can't parse rate: %s", err)
			fmt.Printf("audio: %s\n", errmsg)
			httpRsp.WriteHeader(http.StatusOK)
			httpRsp.Write([]byte(errmsg))
			return
		}
		codec, err := codec2.NewCodec2()
		if err != nil {
			errmsg := fmt.Sprintf("can't instantiate codec2: %s", err)
			fmt.Printf("audio: %s\n", errmsg)
			httpRsp.WriteHeader(http.StatusOK)
			httpRsp.Write([]byte(errmsg))
			return
		}
		c2data = pcmData
		pcmData = []byte{}
		for i := 0; i < len(c2data); i += codec2.BytesPerFrame {
			end := i + codec2.BytesPerFrame
			if end > len(c2data) {
				break // Don't process partial frames
			}
			frame := c2data[i:end]
			pcm, err := codec.Decode(frame)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding frame %d: %v\n", i/codec2.BytesPerFrame+1, err)
				os.Exit(1)
			}
			decodedFrame := make([]byte, codec2.SamplesPerFrame*2)
			for j := 0; j < codec2.SamplesPerFrame; j++ {
				binary.LittleEndian.PutUint16(decodedFrame[j*2:], uint16(pcm[j]))
			}
			pcmData = append(pcmData, decodedFrame...)
		}
	} else {
		errmsg := fmt.Sprintf("unsupported media type: %s", mediatype)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}

	// Convert PCM data to WAV
	wavData, err := PCMToWAV(pcmData, 16, rate)
	if err != nil {
		errmsg := fmt.Sprintf("can't convert to wav: %s", err)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}

	// Convert the audio to text
	fmt.Printf("audio: %d bytes to be sent to %s %s %s:\n", len(pcmData), responseDeviceUID, responseProductUID, responseNotefileID)

	// Print the pcm slice in an 8-column display.
	// Each value is printed as a right-aligned, fixed-width (10 characters) decimal.
	pcm := make([]int16, len(pcmData)/2)
	for i := 0; i < len(pcm); i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(pcmData[i*2 : i*2+2]))
	}
	for i, sample := range pcm {
		fmt.Printf("%10d", sample)
		// After every 8 values, add a new line.
		if (i+1)%8 == 0 {
			fmt.Println()
			if i > 64 {
				break
			}
		}
	}

	// Write the audio to files
	filetime := time.Now().UTC().Unix()
	if len(c2data) != 0 {
		filename := fmt.Sprintf("audio/%d.c2", filetime)
		err = os.WriteFile(configDataDirectory+filename, c2data, 0644)
		if err != nil {
			fmt.Println("Error writing file:", err)
		} else {
			fmt.Printf("https://myjson.live/%s\n", filename)
		}
	}
	filename := fmt.Sprintf("audio/%d.raw", filetime)
	err = os.WriteFile(configDataDirectory+filename, pcmData, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	} else {
		fmt.Printf("https://myjson.live/%s\n", filename)
	}
	filename = fmt.Sprintf("audio/%d.wav", filetime)
	err = os.WriteFile(configDataDirectory+filename, wavData, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	} else {
		fmt.Printf("https://myjson.live/%s\n", filename)
	}

	// Done
	httpRsp.WriteHeader(http.StatusOK)

}

// MemWriteSeeker is an in-memory implementation of io.WriteSeeker.
type MemWriteSeeker struct {
	buf []byte
	pos int64
}

// NewMemWriteSeeker returns a new MemWriteSeeker.
func NewMemWriteSeeker() *MemWriteSeeker {
	return &MemWriteSeeker{
		buf: make([]byte, 0),
		pos: 0,
	}
}

// Write writes p into the buffer, expanding it if necessary.
func (m *MemWriteSeeker) Write(p []byte) (int, error) {
	endPos := int(m.pos) + len(p)
	if endPos > len(m.buf) {
		// Expand the buffer. If writing past the current end, fill the gap.
		if m.pos > int64(len(m.buf)) {
			return 0, fmt.Errorf("invalid write position")
		}
		newBuf := make([]byte, endPos)
		copy(newBuf, m.buf)
		m.buf = newBuf
	}
	copy(m.buf[m.pos:], p)
	m.pos += int64(len(p))
	return len(p), nil
}

// Seek sets the offset for the next Write.
func (m *MemWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = m.pos + offset
	case io.SeekEnd:
		newPos = int64(len(m.buf)) + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if newPos < 0 {
		return 0, errors.New("negative position")
	}
	m.pos = newPos
	return newPos, nil
}

// Bytes returns the complete written buffer.
func (m *MemWriteSeeker) Bytes() []byte {
	return m.buf
}

// PCMToWAV converts a PCM payload (little-endian, mono, 16-bit) to a WAV file in memory.
// It takes the PCM data as a byte slice, the bit depth (e.g. 16), and the sample rate (e.g. 8000),
// and returns the WAV data as a byte slice.
func PCMToWAV(pcmData []byte, bitDepth int, sampleRate int) ([]byte, error) {
	// For 16-bit PCM, each sample is 2 bytes.
	if len(pcmData)%(bitDepth/8) != 0 {
		return nil, fmt.Errorf("invalid PCM data length: must be a multiple of %d", bitDepth/8)
	}

	numSamples := len(pcmData) / (bitDepth / 8)
	samples := make([]int, numSamples)

	// Convert each sample from PCM (assuming 16-bit little-endian).
	for i := 0; i < numSamples; i++ {
		offset := i * 2
		sample := int16(binary.LittleEndian.Uint16(pcmData[offset : offset+2]))
		samples[i] = int(sample)
	}

	// Create an audio buffer.
	buffer := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  sampleRate,
		},
		Data:           samples,
		SourceBitDepth: bitDepth,
	}

	// Use MemWriteSeeker instead of bytes.Buffer to satisfy io.WriteSeeker.
	memWS := NewMemWriteSeeker()
	encoder := wav.NewEncoder(memWS, sampleRate, bitDepth, 1, 1)

	if err := encoder.Write(buffer); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return memWS.Bytes(), nil
}
