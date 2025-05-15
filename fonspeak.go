package fonspeak

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

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
	Filename  string
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

func FonspeakSyllable(params FonParams) error {
	cmd := exec.Command("espeak-ng", "-v", params.Voice, "-w", params.WavFile, "-z", fmt.Sprintf("[[%s]]", params.Syllable))
	if err := cmd.Run(); err != nil {
		return err
	}
	pitchShift(params.WavFile, params.PitchShift)

	return nil
}

func FonspeakPhrase(params PhraseParams) error {
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
		err = FonspeakSyllable(fpr)
		if err != nil {
			return err
		}

		waves = append(waves, fpr.WavFile)
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
