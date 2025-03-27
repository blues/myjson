// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	codec2 "github.com/blues/codec2/go"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

// Protocol of the "body" field in Events
type AudioRequest struct {
	Id          uint64 `json:"id,omitempty"`
	ContentType string `json:"content,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	Last        bool   `json:"last,omitempty"`
	ReplyMax    int    `json:"reply_max,omitempty"`
}
type AudioResponse struct {
	Id          uint64 `json:"id,omitempty"`
	ContentType string `json:"content,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	Last        bool   `json:"last,omitempty"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
	Payload     []byte `json:"payload,omitempty"`
}

const c2ContentType = "audio/codec2-2400;rate=8000"

// Cache of audio data being uploaded, indexed by deviceUID
var audioCache map[string][]byte = map[string][]byte{}
var audioCacheLock sync.RWMutex

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
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err == nil {
			httpRsp.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		} else {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		httpRsp.Header().Set("Content-Type", "application/octet-stream")
		httpRsp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		httpRsp.WriteHeader(http.StatusOK)

		_, err = io.Copy(httpRsp, file)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}

		return
	}

	// If this is an event, process it differently than if it's raw audio
	contentType := httpReq.Header.Get("Content-Type")
	if contentType == "application/json" {

		// Read the JSON event which is an Event structure
		eventJSON, err := io.ReadAll(httpReq.Body)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		var event note.Event
		err = note.JSONUnmarshal(eventJSON, &event)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		c2Data := event.Payload

		// Read the request JSON which is a request structure
		var request AudioRequest
		if event.Body == nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte("no body in event"))
			return
		}
		bodyJSON, err := note.ObjectToJSON(*event.Body)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		err = note.JSONUnmarshal(bodyJSON, &request)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}

		// Multi-segment upload cache management
		audioCacheLock.Lock()
		if request.Offset == 0 {
			audioCache[event.DeviceUID] = c2Data
			fmt.Printf("audio: %s: first %d bytes\n", event.DeviceUID, len(c2Data))
		} else if request.Offset == len(audioCache[event.DeviceUID]) {
			audioCache[event.DeviceUID] = append(audioCache[event.DeviceUID], c2Data...)
			fmt.Printf("audio: %s: appended %d bytes\n", event.DeviceUID, len(c2Data))
		} else {
			errstr := fmt.Sprintf("offset mismatch: %d != %d", request.Offset, len(audioCache[event.DeviceUID]))
			fmt.Printf("audio: %s: %s\n", event.DeviceUID, errstr)
			audioCacheLock.Unlock()
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(errstr))
			return
		}
		if !request.Last {
			audioCacheLock.Unlock()
			httpRsp.WriteHeader(http.StatusOK)
			return
		}

		c2Data = audioCache[event.DeviceUID]
		delete(audioCache, event.DeviceUID)
		fmt.Printf("audio: %s: processing %d bytes\n", event.DeviceUID, len(c2Data))
		audioCacheLock.Unlock()

		// Process the data
		err = processAudioRequest(httpReq, event, request, c2Data)
		if err != nil {
			fmt.Printf("audio error: %s\n", err)
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
		} else {
			httpRsp.WriteHeader(http.StatusOK)
		}
		return
	}

	// Remove the 1 byte of padding added by the notecard to "complete" the last page
	pcmData, _ := io.ReadAll(httpReq.Body)
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
		c2data = pcmData
		pcmData, err = C2ToPCM(c2data, rate)
		if err != nil {
			errmsg := fmt.Sprintf("can't convert codec2 to pcm: %s", err)
			fmt.Printf("audio: %s\n", errmsg)
			httpRsp.WriteHeader(http.StatusOK)
			httpRsp.Write([]byte(errmsg))
			return
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
	fmt.Printf("audio: %d bytes\n", len(pcmData))

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

// C2ToPCM converts a Codec2 payload to PCM (16-bit, little-endian) in memory.
func C2ToPCM(c2data []byte, rate int) (pcmData []byte, err error) {
	codec, err := codec2.NewCodec2()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(c2data); i += codec2.BytesPerFrame {
		end := i + codec2.BytesPerFrame
		if end > len(c2data) {
			break // Don't process partial frames
		}
		frame := c2data[i:end]
		pcm, err := codec.Decode(frame)
		if err != nil {
			return nil, err
		}
		decodedFrame := make([]byte, codec2.SamplesPerFrame*2)
		for j := 0; j < codec2.SamplesPerFrame; j++ {
			binary.LittleEndian.PutUint16(decodedFrame[j*2:], uint16(pcm[j]))
		}
		pcmData = append(pcmData, decodedFrame...)
	}

	return pcmData, nil
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

// Process an event request/response
func processAudioRequest(httpReq *http.Request, event note.Event, request AudioRequest, c2Data []byte) (err error) {

	// Extract required parameters from the Route configuration
	header := "X-OpenAiApiKey"
	openAiApiKey := httpReq.Header.Get(header)
	if openAiApiKey == "" {
		return fmt.Errorf("%s not specified", header)
	}
	header = "X-RequestNotefile"
	requestNotefileID := httpReq.Header.Get(header)
	if requestNotefileID == "" {
		return fmt.Errorf("%s not specified", header)
	}
	header = "X-ResponseNotefile"
	responseNotefileID := httpReq.Header.Get(header)
	if responseNotefileID == "" {
		return fmt.Errorf("%s not specified", header)
	}
	header = "X-ResponseToken"
	responseApiToken := httpReq.Header.Get(header)
	if responseApiToken == "" {
		return fmt.Errorf("%s not specified", header)
	}

	// Read the C2 data from the payload, and convert it to WAV
	rate := 8000
	pcmData, err := C2ToPCM(c2Data, rate)
	if err != nil {
		return err
	}
	wavData, err := PCMToWAV(pcmData, 16, rate)
	if err != nil {
		return err
	}

	// For debugging
	filetime := time.Now().UTC().Unix()
	filename := fmt.Sprintf("audio/%d.wav", filetime)
	err = os.WriteFile(configDataDirectory+filename, wavData, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	} else {
		fmt.Printf("https://myjson.live/%s\n", filename)
	}

	// Return the request ID in the response
	var response AudioResponse
	response.Id = request.Id

	// Get the text
	response.Request, response.Response, err = getTextRequestResponseFromWav(openAiApiKey, wavData)
	if err != nil {
		return err
	}

	// If this last segment requested an audio reply, send it before the final reply
	if request.ContentType == c2ContentType {

		// Convert the response to WAV
		var wavDataResponse []byte
		wavDataResponse, err = getWavFromResponse(openAiApiKey, response.Response)
		if err != nil {
			return err
		}

		// Convert the wav to PCM
		var pcmDataResponse []byte
		pcmDataResponse, err = WAVToPCM8k(wavDataResponse)
		if err != nil {
			return err
		}

		// Convert the PCM to codec2
		c2DataResponse, err := PCMToC2(pcmDataResponse)
		if err != nil {
			return err
		}
		fmt.Printf("audio: response WAV/PCM/C2 is %d/%d/%d bytes\n", len(wavDataResponse), len(pcmDataResponse), len(c2DataResponse))

		// Send potentially-numerous chunks to the Notecard
		maxChunkSize := request.ReplyMax
		if maxChunkSize == 0 {
			maxChunkSize = 8192
		}

		// Send the chunked payload response to the Notecard, all intended to arrive
		// before the final response containing the body
		offset := 0
		for {
			chunkSize := maxChunkSize
			if len(c2DataResponse) < maxChunkSize {
				chunkSize = len(c2DataResponse)
			}
			if chunkSize == 0 {
				break
			}
			var chunk []byte
			chunk, c2DataResponse = c2DataResponse[:chunkSize], c2DataResponse[chunkSize:]

			var rsp AudioResponse
			rsp.Id = request.Id
			rsp.Payload = chunk
			rsp.Offset = offset
			rsp.ContentType = c2ContentType
			rspJSON, err := note.ObjectToJSON(rsp)
			if err != nil {
				return err
			}
			body, err := note.JSONToBody(rspJSON)
			if err != nil {
				return err
			}
			hubreq := notehub.HubRequest{}
			hubreq.Req = "note.add"
			hubreq.Body = &body
			hubreq.NotefileID = responseNotefileID
			hubreq.AppUID = event.AppUID
			hubreq.DeviceUID = event.DeviceUID
			hubreqJSON, err := note.JSONMarshal(hubreq)
			if err != nil {
				return err
			}
			fmt.Printf("audio: request: %s\n", hubreqJSON)

			hreq, _ := http.NewRequest("POST", "https://"+notehub.DefaultAPIService, bytes.NewBuffer(hubreqJSON))
			hreq.Header.Set("User-Agent", "audio request processor")
			hreq.Header.Set("Content-Type", "application/json")
			hreq.Header.Set("X-Session-Token", responseApiToken)
			httpClient := &http.Client{Timeout: time.Second * 10}
			hrsp, err := httpClient.Do(hreq)
			if err != nil {
				return err
			}
			hubrspJSON, err := io.ReadAll(hrsp.Body)
			if err != nil {
				return err
			}
			fmt.Printf("audio: response: len:%d %s\n", len(chunk), hubrspJSON)

			offset += len(chunk)
		}

	}

	// Convert the (final) response to JSON
	responseJSON, err := note.ObjectToJSON(response)
	if err != nil {
		return err
	}
	body, err := note.JSONToBody(responseJSON)
	if err != nil {
		return err
	}

	// Send the response to the app
	hubreq := notehub.HubRequest{}
	hubreq.Req = "note.add"
	hubreq.Body = &body
	hubreq.NotefileID = responseNotefileID
	hubreq.AppUID = event.AppUID
	hubreq.DeviceUID = event.DeviceUID
	hubreqJSON, err := note.JSONMarshal(hubreq)
	if err != nil {
		return err
	}
	fmt.Printf("audio: request: %s\n", hubreqJSON)

	hreq, _ := http.NewRequest("POST", "https://"+notehub.DefaultAPIService, bytes.NewBuffer(hubreqJSON))
	hreq.Header.Set("User-Agent", "audio request processor")
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("X-Session-Token", responseApiToken)
	httpClient := &http.Client{Timeout: time.Second * 10}
	hrsp, err := httpClient.Do(hreq)
	if err != nil {
		return err
	}
	hubrspJSON, err := io.ReadAll(hrsp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("audio: response: %s\n", hubrspJSON)

	// Done
	return nil

}

// getTextRequestResponse calls the OpenAI Whisper API to transcribe the audio,
// and the ChatGPT API to generate a response.
func getTextRequestResponseFromWav(openAiApiKey string, wavData []byte) (request string, response string, err error) {
	// --- Call OpenAI Whisper API for transcription ---
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Create a form file field "file" with the WAV data
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", "", fmt.Errorf("failed to create form file: %v", err)
	}
	if _, err = part.Write(wavData); err != nil {
		return "", "", fmt.Errorf("failed to write wav data: %v", err)
	}

	// Add required form field for the Whisper model.  Someday we can even
	// add a "prompt=" field to provide context for the transcription.
	// See this.https://platform.openai.com/docs/guides/speech-to-text
	//	if err = writer.WriteField("model", "whisper-1"); err != nil {
	if err = writer.WriteField("model", "gpt-4o-mini-transcribe"); err != nil {
		return "", "", fmt.Errorf("failed to write model field: %v", err)
	}
	if err = writer.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close writer: %v", err)
	}

	// Build the transcription API request
	transcriptionURL := "https://api.openai.com/v1/audio/transcriptions"
	transcriptionReq, err := http.NewRequest("POST", transcriptionURL, &b)
	if err != nil {
		return "", "", fmt.Errorf("failed to create transcription request: %v", err)
	}
	transcriptionReq.Header.Set("Content-Type", writer.FormDataContentType())
	transcriptionReq.Header.Set("Authorization", "Bearer "+openAiApiKey)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	transcriptionResp, err := httpClient.Do(transcriptionReq)
	if err != nil {
		return "", "", fmt.Errorf("failed to perform transcription request: %v", err)
	}
	defer transcriptionResp.Body.Close()

	if transcriptionResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(transcriptionResp.Body)
		return "", "", fmt.Errorf("transcription API error: %s", string(bodyBytes))
	}

	// Parse the transcription response (expected JSON: { "text": "..." })
	var transcriptionResult struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(transcriptionResp.Body).Decode(&transcriptionResult); err != nil {
		return "", "", fmt.Errorf("failed to decode transcription response: %v", err)
	}
	transcription := transcriptionResult.Text

	// --- Call ChatGPT API for ASCII text response ---
	chatPayload := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": transcription,
			},
		},
	}
	payloadBytes, err := json.Marshal(chatPayload)
	if err != nil {
		return transcription, "", fmt.Errorf("failed to marshal chat payload: %v", err)
	}

	chatURL := "https://api.openai.com/v1/chat/completions"
	chatReq, err := http.NewRequest("POST", chatURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return transcription, "", fmt.Errorf("failed to create chat request: %v", err)
	}
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.Header.Set("Authorization", "Bearer "+openAiApiKey)

	chatResp, err := httpClient.Do(chatReq)
	if err != nil {
		return transcription, "", fmt.Errorf("failed to perform chat request: %v", err)
	}
	defer chatResp.Body.Close()

	if chatResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(chatResp.Body)
		return transcription, "", fmt.Errorf("chat API error: %s", string(bodyBytes))
	}

	var chatResult struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(chatResp.Body).Decode(&chatResult); err != nil {
		return transcription, "", fmt.Errorf("failed to decode chat response: %v", err)
	}
	if len(chatResult.Choices) == 0 {
		return transcription, "", fmt.Errorf("chat response missing choices")
	}

	asciiResponse := chatResult.Choices[0].Message.Content

	// Return transcription as the request and ChatGPT's response as the response.
	return transcription, asciiResponse, nil
}

// getWavFromResponse converts text (from the ChatGPT response) back into WAV audio using OpenAI’s Text-to-Speech API.
func getWavFromResponse(openAiApiKey string, text string) (wavData []byte, err error) {
	// Build the JSON payload as per the OpenAI API Reference for createSpeech.
	payload := map[string]interface{}{
		"input":           text,
		"response_format": "wav",
		"model":           "gpt-4o-mini-tts",
		"voice":           "coral",
		"instructions":    "Speak in a cheerful and positive tone.",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TTS payload: %v", err)
	}

	ttsURL := "https://api.openai.com/v1/audio/speech"
	ttsReq, err := http.NewRequest("POST", ttsURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS request: %v", err)
	}
	ttsReq.Header.Set("Content-Type", "application/json")
	ttsReq.Header.Set("Authorization", "Bearer "+openAiApiKey)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	ttsResp, err := httpClient.Do(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to perform TTS request: %v", err)
	}
	defer ttsResp.Body.Close()

	if ttsResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(ttsResp.Body)
		return nil, fmt.Errorf("TTS API error: %s", string(bodyBytes))
	}

	// Read and return the WAV data from the response
	wavData, err = io.ReadAll(ttsResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS response: %v", err)
	}
	return wavData, nil
}

// WAVToPCM8k converts WAV data to a PCM []byte array at 8kHz.
// It decodes the WAV, and if the sample rate is not 8000, uses linear interpolation to downsample.
func WAVToPCM8k(wavData []byte) ([]byte, error) {
	r := bytes.NewReader(wavData)
	decoder := wav.NewDecoder(r)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}
	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, err
	}
	// Ensure we have an IntBuffer.
	intBuf := buf.AsIntBuffer()
	inputRate := intBuf.Format.SampleRate

	var resampled []int
	if inputRate == 8000 {
		resampled = intBuf.Data
	} else {
		resampled = resampleLinear(intBuf.Data, int(inputRate), 8000)
	}
	pcmBytes := make([]byte, len(resampled)*2)
	for i, sample := range resampled {
		binary.LittleEndian.PutUint16(pcmBytes[i*2:], uint16(sample))
	}
	return pcmBytes, nil
}

// resampleLinear performs a simple linear interpolation to convert sample rate.
func resampleLinear(samples []int, srcRate int, dstRate int) []int {
	newLength := int(float64(len(samples)) * float64(dstRate) / float64(srcRate))
	resampled := make([]int, newLength)
	for i := 0; i < newLength; i++ {
		srcIndex := float64(i) * float64(srcRate) / float64(dstRate)
		indexInt := int(srcIndex)
		frac := srcIndex - float64(indexInt)
		if indexInt+1 < len(samples) {
			resampled[i] = int(float64(samples[indexInt])*(1-frac) + float64(samples[indexInt+1])*frac)
		} else {
			resampled[i] = samples[indexInt]
		}
	}
	return resampled
}

// PCMToC2 converts a raw 16-bit PCM []byte (mono, little-endian) into Codec2 encoded data.
// If the length of pcmData is not a multiple of codec2.SamplesPerFrame*2, it zero-fills the remaining bytes.
func PCMToC2(pcmData []byte) ([]byte, error) {
	codec, err := codec2.NewCodec2()
	if err != nil {
		return nil, err
	}

	frameSize := codec2.SamplesPerFrame * 2 // bytes per frame
	// Pad pcmData with zeros if necessary
	remainder := len(pcmData) % frameSize
	if remainder != 0 {
		paddingSize := frameSize - remainder
		pcmData = append(pcmData, make([]byte, paddingSize)...)
	}

	var c2Data []byte
	for i := 0; i < len(pcmData); i += frameSize {
		pcmFrame := make(codec2.PCMBuffer, codec2.SamplesPerFrame)
		for j := 0; j < codec2.SamplesPerFrame; j++ {
			offset := i + j*2
			pcmFrame[j] = int16(binary.LittleEndian.Uint16(pcmData[offset : offset+2]))
		}

		encodedFrame, err := codec.Encode(pcmFrame)
		if err != nil {
			return nil, err
		}
		c2Data = append(c2Data, encodedFrame...)
	}
	return c2Data, nil
}
