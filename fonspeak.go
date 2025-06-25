package fonspeak

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sync"

	"github.com/go-audio/wav"
)

type SyllableResult struct {
	Message string
	Error   error
}

type Params struct {
	Syllable   string
	PitchShift float64
	Voice      string
}

type FonParams struct {
	Params
	WavFile string
}

type PhraseParams struct {
	Syllables []Params
	WavFile   io.WriteCloser
}

func pitchShift(wave string, shift float64) error {
	waveout := wave + "_out.wav"
	f, err := os.Open(wave)
	if err != nil {
		return err
	}

	defer f.Close()

	decoder := wav.NewDecoder(f)
	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return err
	}

	var audioData []float64

	numSamples := len(buf.Data) / buf.Format.NumChannels
	audioData = make([]float64, numSamples)

	for i := range numSamples {
		// For mono, or just taking the first channel of stereo
		audioData[i] = float64(buf.Data[i*buf.Format.NumChannels]) / math.Pow(2, float64(buf.SourceBitDepth-1)-1)
	}
	fmt.Printf("Read %d samples from WAV file.\n", len(audioData))

	originalF0, err := estimateFundamentalFrequency(audioData, int(decoder.SampleRate))
	if err != nil {
		return fmt.Errorf("error estimating original F0: %v", err)
	}

	fmt.Printf("Estimated original F0: %.2f Hz\n", originalF0)

	pitchRatio := shift / originalF0

	fmt.Printf("PitchRatio: %.2f\n", pitchRatio)

	cmd := exec.CommandContext(context.Background(), "rubberband-r3", "-t", "1.0", "-p", fmt.Sprintf("%.1f", pitchRatio), wave, waveout)
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := os.Rename(waveout, wave); err != nil {
		return err
	}

	return nil
}

func FonspeakSyllable(params FonParams) error {
	cmd := exec.CommandContext(context.Background(), "espeak-ng", "-v", params.Voice, "-w", params.WavFile, "-z", fmt.Sprintf("[[%s]]", params.Syllable))
	if err := cmd.Run(); err != nil {
		return err
	}

	err := pitchShift(params.WavFile, params.PitchShift)
	if err != nil {
		return err
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

	// defer os.RemoveAll(dir)

	var waves []string

	for i, pr := range params.Syllables {
		fpr := FonParams{
			Params:  pr,
			WavFile: fmt.Sprintf("%s/%d.wav", dir, i),
		}
		waves = append(waves, fpr.WavFile)
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
		return err
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
