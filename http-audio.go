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
	"math"
	"math/rand"
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
	Id               uint64 `json:"id,omitempty"`
	ContentType      string `json:"content,omitempty"`
	Offset           int    `json:"offset,omitempty"`
	Last             bool   `json:"last,omitempty"`
	Voice            string `json:"voice,omitempty"`
	ReplyContentType string `json:"reply_content,omitempty"`
	ReplyMax         int    `json:"reply_max,omitempty"`
}
type AudioResponse struct {
	Id          uint64 `json:"id,omitempty"`
	ContentType string `json:"content,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	Length      int    `json:"length,omitempty"`
	Total       int    `json:"total,omitempty"`
	Voice       string `json:"voice,omitempty"`
	Last        bool   `json:"last,omitempty"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
}

const contentTypeC2 = "audio/codec2-2400;rate=8000"
const contentTypePCM = "audio/L16;rate=8000"

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
		pcmData, err = C2ToPCM8K(c2data, rate)
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

	// Convert PCM8K data to WAV
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

// C2ToPCM8K converts a Codec2 payload to PCM8K (16-bit, little-endian) in memory.
func C2ToPCM8K(c2data []byte, rate int) (pcmData []byte, err error) {
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

// PCMToWAV converts a PCM8K payload (little-endian, mono, 16-bit) to a WAV file in memory.
// It takes the PCM8K data as a byte slice, the bit depth (e.g. 16), and the sample rate (e.g. 8000),
// and returns the WAV data as a byte slice.
func PCMToWAV(pcmData []byte, bitDepth int, sampleRate int) ([]byte, error) {
	// For 16-bit PCM8K, each sample is 2 bytes.
	if len(pcmData)%(bitDepth/8) != 0 {
		return nil, fmt.Errorf("invalid PCM8K data length: must be a multiple of %d", bitDepth/8)
	}

	numSamples := len(pcmData) / (bitDepth / 8)
	samples := make([]int, numSamples)

	// Convert each sample from PCM8K (assuming 16-bit little-endian).
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
	pcmData, err := C2ToPCM8K(c2Data, rate)
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

	// If there is no reply needed, send the response now
	if request.ReplyContentType == "" {

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
	}

	// If this last segment requested an audio reply, send it before the final reply
	if request.ReplyContentType == contentTypeC2 || request.ReplyContentType == contentTypePCM {

		// https://platform.openai.com/docs/guides/text-to-speech
		// The voices currently supported by this model are:
		voice := request.Voice
		if voice == "" {
			voices := []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer"}
			voice = voices[rand.Intn(len(voices))]
			fmt.Printf("using voice %s\n", voice)
		}

		// Convert the response to WAV
		var pcm24kDataResponse []byte
		pcm24kDataResponse, err = getPCM24KFromResponse(openAiApiKey, voice, response.Response)
		if err != nil {
			return err
		}

		// Convert the PCM24k to PCM8K
		var pcm8kDataResponse []byte
		pcm8kDataResponse, err = PCM24KToPCM8K(pcm24kDataResponse)
		if err != nil {
			return err
		}

		// Convert the PCM8K to codec2
		dataResponse := pcm8kDataResponse
		var c2DataResponse []byte
		if request.ReplyContentType == contentTypeC2 {
			c2DataResponse, err = PCM8KToC2(pcm8kDataResponse)
			if err != nil {
				return err
			}
			dataResponse = c2DataResponse
		}

		// For debugging, write a wav file
		wavDataResponse, err := PCMToWAV(pcm8kDataResponse, 16, 8000)
		if err == nil {
			filename := "audio/reply.raw"
			os.WriteFile(configDataDirectory+filename, pcm8kDataResponse, 0644)
			fmt.Printf("https://myjson.live/%s\n", filename)
			filename = "audio/reply.wav"
			os.WriteFile(configDataDirectory+filename, wavDataResponse, 0644)
			fmt.Printf("https://myjson.live/%s\n", filename)
			filename = "audio/reply.c2"
			os.WriteFile(configDataDirectory+filename, c2DataResponse, 0644)
			fmt.Printf("https://myjson.live/%s\n", filename)
		}

		// Send potentially-numerous chunks to the Notecard
		maxChunkSize := request.ReplyMax
		if maxChunkSize == 0 {
			maxChunkSize = 8192
		}

		// Send the chunked payload response to the Notecard, all intended to arrive
		// before the final response containing the body
		offset := 0
		total := len(dataResponse)
		for {
			chunkSize := maxChunkSize
			if len(dataResponse) < maxChunkSize {
				chunkSize = len(dataResponse)
			}
			if chunkSize == 0 {
				break
			}
			var chunk []byte
			chunk, dataResponse = dataResponse[:chunkSize], dataResponse[chunkSize:]

			var rsp AudioResponse
			rsp.Id = request.Id
			rsp.Offset = offset
			rsp.Length = len(chunk)
			rsp.Total = total
			rsp.Voice = voice
			rsp.ContentType = request.ReplyContentType
			if len(dataResponse) == 0 {
				rsp.Last = true
				rsp.Request = response.Request
				rsp.Response = response.Response
			}
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
			hubreq.Payload = &chunk
			hubreq.Allow = true
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

			// Intentionally delay between chunks so that the notes on the receiving end are
			// ordered correctly by epoch time.
			time.Sleep(1500 * time.Millisecond)
		}

	}

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
			{"role": "system", "content": "Answer as briefly as possible."},
			{"role": "user", "content": transcription},
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

// getPCM24KFromResponse converts text (from the ChatGPT response) back into audio using OpenAI’s Text-to-Speech API.
func getPCM24KFromResponse(openAiApiKey string, voice string, text string) (wavData []byte, err error) {
	// Build the JSON payload as per the OpenAI API Reference for createSpeech.
	payload := map[string]interface{}{
		"input":           text,
		"response_format": "pcm",
		"model":           "gpt-4o-mini-tts",
		"voice":           voice,
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

// PCM8KToC2 converts a raw 16-bit PCM8K []byte (mono, little-endian) into Codec2 encoded data.
// If the length of pcmData is not a multiple of codec2.SamplesPerFrame*2, it zero-fills the remaining bytes.
func PCM8KToC2(pcmData []byte) ([]byte, error) {
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

// PCM24KToPCM8K downsamples 16-bit PCM audio from 24kHz to 8kHz
// pcmData is expected to be a byte slice containing 16-bit PCM samples
// Returns the downsampled audio as a byte slice and an error if any
func PCM24KToPCM8K(pcmData []byte) ([]byte, error) {
	// Check if data length is valid (must be even for 16-bit samples)
	if len(pcmData)%2 != 0 {
		return nil, errors.New("invalid PCM data: length must be even")
	}

	// Convert bytes to int16 samples
	numSamples := len(pcmData) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		// Convert two bytes to one int16 sample (little-endian)
		samples[i] = int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
	}

	// Design a low-pass filter (FIR filter)
	// Cutoff frequency: 3500 Hz (slightly below Nyquist for 8kHz)
	filterOrder := 64 // Higher order = better quality but more CPU
	filter := designLowPassFilter(filterOrder, 3500.0, 24000.0)

	// Apply filter to prevent aliasing
	filteredSamples := applyFilter(samples, filter)

	// Decimate by factor of 3 (24kHz to 8kHz)
	decimationFactor := 3
	outputLength := numSamples / decimationFactor
	outputSamples := make([]int16, outputLength)

	for i := 0; i < outputLength; i++ {
		outputSamples[i] = filteredSamples[i*decimationFactor]
	}

	// Convert back to bytes
	outputBytes := make([]byte, outputLength*2)
	for i, sample := range outputSamples {
		// Convert each int16 sample to two bytes (little-endian)
		outputBytes[i*2] = byte(sample)
		outputBytes[i*2+1] = byte(sample >> 8)
	}

	return outputBytes, nil
}

// designLowPassFilter creates a FIR low-pass filter using the windowed-sinc method
func designLowPassFilter(order int, cutoffFreq, sampleRate float64) []float64 {
	// Ensure odd filter order for symmetry
	if order%2 == 0 {
		order++
	}

	filter := make([]float64, order)

	// Normalized cutoff frequency (0 to 0.5)
	omega := 2.0 * math.Pi * cutoffFreq / sampleRate

	// Calculate filter coefficients
	halfOrder := order / 2
	for i := 0; i < order; i++ {
		// Sinc function
		n := float64(i - halfOrder)
		if n == 0 {
			filter[i] = omega / math.Pi
		} else {
			filter[i] = math.Sin(omega*n) / (math.Pi * n)
		}

		// Apply Blackman window for better frequency response
		filter[i] *= 0.42 - 0.5*math.Cos(2.0*math.Pi*float64(i)/float64(order-1)) +
			0.08*math.Cos(4.0*math.Pi*float64(i)/float64(order-1))
	}

	// Normalize filter gain
	sum := 0.0
	for _, coeff := range filter {
		sum += coeff
	}
	for i := range filter {
		filter[i] /= sum
	}

	return filter
}

// applyFilter convolves the input samples with the filter coefficients
func applyFilter(samples []int16, filter []float64) []int16 {
	filterLength := len(filter)
	samplesLength := len(samples)
	result := make([]int16, samplesLength)

	for i := 0; i < samplesLength; i++ {
		var sum float64
		for j := 0; j < filterLength; j++ {
			// Use zero padding for samples outside the range
			idx := i - j + filterLength/2
			if idx >= 0 && idx < samplesLength {
				sum += float64(samples[idx]) * filter[j]
			}
		}
		// Convert back to int16 with proper rounding and clamping
		if sum > 32767 {
			result[i] = 32767
		} else if sum < -32768 {
			result[i] = -32768
		} else {
			result[i] = int16(math.Round(sum))
		}
	}

	return result
}
