package fonspeak

import (
	"context"
	"embed"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"io"
	"os"
	"os/exec"
	"sync"
)

type SyllableResult struct {
	Message string
	Error   error
}

type Params struct {
	Syllable   string
	PitchShift float64
	Voice      string
	Wpm        int
}

type FonParams struct {
	Params
	WavFile string
}

type PhraseParams struct {
	Syllables []Params
	WavFile   io.WriteCloser
}

type FloorParams struct {
	SegLenSec     float64 // phoneme duration in seconds
	DesiredPeriods float64 // e.g., 3.0
	DefaultWinLen  float64 // e.g., 0.04
	MinFloorHz     float64 // e.g., 85
	MaxFloorHz     float64 // e.g., 300
}

//go:embed pitch.praat
var content embed.FS

func calculateDuration(wav string) float64 {
	f, err := os.Open(wav)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer f.Close()
	
	var (
		sampleRate   uint32
		numChannels  uint16
		bitsPerSample uint16
		dataSize     uint32
	)

	// Read RIFF header (12 bytes)
	hdr := make([]byte, 12)
	if _, err := f.Read(hdr); err != nil {
		log.Fatalf("read header: %v", err)
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		log.Fatal("not a RIFF/WAVE file")
	}

	// Iterate chunks until we find "fmt " and "data"
	for {
		chdr := make([]byte, 8)
		if _, err := f.Read(chdr); err != nil {
			log.Fatalf("read chunk header: %v", err)
		}
		id := string(chdr[0:4])
		size := binary.LittleEndian.Uint32(chdr[4:8])

		switch id {
		case "fmt ":
			fmtChunk := make([]byte, size)
			if _, err := f.Read(fmtChunk); err != nil {
				log.Fatalf("read fmt chunk: %v", err)
			}
			// PCM fmt chunk layout
			// 0..1: audio format (1=PCM)
			// 2..3: numChannels
			// 4..7: sampleRate
			// 8..11: byteRate
			// 12..13: blockAlign
			// 14..15: bitsPerSample
			numChannels = binary.LittleEndian.Uint16(fmtChunk[2:4])
			sampleRate = binary.LittleEndian.Uint32(fmtChunk[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(fmtChunk[14:16])
		case "data":
			dataSize = size
			// We can stop after reading data header; skip data bytes
			if _, err := f.Seek(int64(size), 1); err != nil {
				log.Fatalf("seek data: %v", err)
			}
		default:
			// Skip other chunks
			if _, err := f.Seek(int64(size), 1); err != nil {
				log.Fatalf("seek chunk: %v", err)
			}
		}

		// Break once we have both fmt and data info
		if sampleRate != 0 && numChannels != 0 && bitsPerSample != 0 && dataSize != 0 {
			break
		}
	}

	bytesPerSample := bitsPerSample / 8
	if bytesPerSample == 0 {
		log.Fatal("invalid bitsPerSample")
	}

	// Duration seconds = dataSize / (sampleRate * bytesPerSample * numChannels)
	durationSec := float64(dataSize) / float64(sampleRate*uint32(bytesPerSample)*uint32(numChannels))
	
	return durationSec
}

func computeFloor(p FloorParams) (floorHz float64, timeStep float64, winLen float64) {
	seg := p.SegLenSec
	if seg <= 0 {
		seg = p.DefaultWinLen
	}
	winLen = math.Min(p.DefaultWinLen, 0.5*seg)
	if winLen < 0.005 {
		winLen = 0.005
	}
	floorHz = p.DesiredPeriods / winLen
	// Clamp to voice-appropriate range
	if floorHz < p.MinFloorHz {
		floorHz = p.MinFloorHz
	}
	if floorHz > p.MaxFloorHz {
		floorHz = p.MaxFloorHz
	}
	// Time step proportional to window (Praat-friendly)
	timeStep = math.Min(0.01, 0.25*winLen)
	if timeStep < 0.0025 {
		timeStep = 0.0025
	}
	return floorHz, timeStep, winLen
}

func pitchShift(wave string, shift float64, phonemeLength float64) error {
	f, err := content.ReadFile("pitch.praat")
	if err != nil {
		return err
	}
	pitcher, err := os.CreateTemp("", "pitch.praat")
	if err != nil {
		return err
	}
	pitcher.Write(f)

	defer pitcher.Close()
	defer os.Remove(pitcher.Name())
	
	floorHz, timeStep, _ := computeFloor(FloorParams{
		SegLenSec:      phonemeLength,
		DesiredPeriods: 3.0,
		DefaultWinLen:  0.04,
		MinFloorHz:     50,
		MaxFloorHz:     400,
	})

	cmd := exec.Command("praat", "--run", "--no-pref-files", "--no-plugins", pitcher.Name(), wave, fmt.Sprintf("%f", shift), fmt.Sprintf("%f", timeStep), fmt.Sprintf("%f", floorHz))

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error running praat: %s ... %w", out, err)
	}

	return nil
}

func FonspeakSyllable(params FonParams) error {
	cmd := exec.CommandContext(context.Background(), "espeak-ng", "-v", params.Voice, "-w", params.WavFile, "-z", fmt.Sprintf("[[%s]]", params.Syllable), "-s", fmt.Sprintf("%d", params.Wpm))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running espeak-ng: %w", err)
	}

	duration := calculateDuration(params.WavFile)

	err := pitchShift(params.WavFile, params.PitchShift, duration)
	if err != nil {
		return fmt.Errorf("error running espeak-ng: %w", err)
	}

	return nil
}

func FonspeakPhrase(params PhraseParams, grMax int) error {
	var wg sync.WaitGroup
	var goErr error
	goroutines := make(chan struct{}, grMax)
	dir, err := os.MkdirTemp("", "phonemes")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dir)

	var waves []string

	for i, pr := range params.Syllables {
		fpr := FonParams{
			Params:  pr,
			WavFile: fmt.Sprintf("%s/%d.wav", dir, i),
		}
		waves = append(waves, fpr.WavFile+"_out.wav")
		goroutines <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-goroutines; wg.Done() }()
			if err := FonspeakSyllable(fpr); err != nil {
				goErr = err
				return
			}
		}()
	}

	wg.Wait()

	if goErr != nil {
		return goErr
	}

	t, err := os.MkdirTemp("", "finished")
	if err != nil {
		return err
	}

	defer os.RemoveAll(t)
	filename := fmt.Sprintf("%s/finished.wav", t)

	waves = append(waves, filename)

	cmd := exec.Command("sox", waves...)
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("error running sox: %w", err)
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer f.Close()

	stats, err := f.Stat()
	if err != nil {
		return err
	}

	b := make([]byte, stats.Size())

	_, err = f.Read(b)
	if err != nil {
		return err
	}

	params.WavFile.Write(b)

	return nil
}
