package fonspeak

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
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
	cmd := exec.Command("python", "-c", fmt.Sprintf(`import librosa;import psola;import argparse;import soundfile as sf;from pathlib import Path;filepath = Path('%s');y, sr = librosa.load(str(filepath), sr=None, mono=False);fmin = librosa.note_to_hz('C1').astype(float);fmax = librosa.note_to_hz('C7').astype(float);corrected_f0 = [%f];pitch_corrected_y = psola.vocode(y, sample_rate=int(sr), target_pitch=corrected_f0, fmin=fmin, fmax=fmax);sf.write(str(filepath), pitch_corrected_y, sr);`, wave, shift))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	fmt.Print(string(out))

	return nil
}

func FonspeakSyllable(params FonParams, wg *sync.WaitGroup, ch chan SyllableResult, ctx context.Context, cancel context.CancelFunc) {
	defer wg.Done()
	cmd := exec.CommandContext(ctx, "espeak-ng", "-v", params.Voice, "-w", params.WavFile, "-z", fmt.Sprintf("[[%s]]", params.Syllable))
	if err := cmd.Start(); err != nil {
		ch <- SyllableResult{
			Message: "Error",
			Error:   err,
		}
		cancel()
	}

	err := pitchShift(params.WavFile, params.PitchShift)
	if err != nil {
		ch <- SyllableResult{
			Message: "Error",
			Error:   err,
		}
		cancel()
	}

	ch <- SyllableResult{
		Message: "Done",
		Error:   nil,
	}
	cancel()
}

func FonspeakPhrase(params PhraseParams) error {
	var wg sync.WaitGroup
	ch := make(chan SyllableResult)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	dir, err := os.MkdirTemp("", "phonemes")
	if err != nil {
		cancel()
		return err
	}

	defer os.RemoveAll(dir)

	var waves []string

	for i, pr := range params.Syllables {
		wg.Add(1)
		fpr := FonParams{
			Params:  pr,
			WavFile: fmt.Sprintf("%s/%d.wav", dir, i),
		}
		waves = append(waves, fpr.WavFile)
		go FonspeakSyllable(fpr, &wg, ch, ctx, cancel)
	}

	for range params.Syllables {
		res := <-ch
		err = res.Error
		if err != nil {
			cancel()
			return err
		}
	}

	wg.Wait()

	t, err := os.MkdirTemp("", "finished")
	if err != nil {
		cancel()
		return err
	}

	defer os.RemoveAll(t)
	filename := fmt.Sprintf("%s/finished.wav", t)

	waves = append(waves, filename)

	cmd := exec.Command("sox", waves...)
	if err = cmd.Start(); err != nil {
		cancel()
		return err
	}

	f, err := os.Open(filename)
	if err != nil {
		cancel()
		return err
	}

	defer f.Close()

	stats, err := f.Stat()
	if err != nil {
		cancel()
		return err
	}

	b := make([]byte, stats.Size())

	_, err = f.Read(b)
	if err != nil {
		cancel()
		return err
	}

	params.WavFile.Write(b)

	cancel()
	return nil
}
