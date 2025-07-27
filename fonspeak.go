package fonspeak

import (
	"context"
	"embed"
	"fmt"
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

//go:embed pitch.praat
var content embed.FS

func pitchShift(wave string, shift float64) error {
	f, err := content.ReadFile("pitch.praat")
	if err != nil {
		return err
	}
	pitcher, err := os.CreateTemp("", "pitch.praat")
	if err != nil {
		return err
	}
	pitcher.Write(f)

	cmd := exec.Command("praat", "--run", "--no-pref-files", "--no-plugins", pitcher.Name(), wave, fmt.Sprintf("%f", shift))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running praat: %w", err)
	}

	return nil
}

func FonspeakSyllable(params FonParams) error {
	cmd := exec.CommandContext(context.Background(), "espeak-ng", "-v", params.Voice, "-w", params.WavFile, "-z", fmt.Sprintf("[[%s]]", params.Syllable), "-s", fmt.Sprintf("%d", params.Wpm))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running espeak-ng: %w", err)
	}

	err := pitchShift(params.WavFile, params.PitchShift)
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
