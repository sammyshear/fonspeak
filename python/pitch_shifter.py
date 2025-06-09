#!/usr/bin/env python
import librosa
import psola
import argparse
import soundfile as sf
from pathlib import Path


def shifter(audio, sr, target_f0):
    fmin = librosa.note_to_hz("C1").astype(float)
    fmax = librosa.note_to_hz("C7").astype(float)

    # Apply the chosen adjustment strategy to the pitch.
    corrected_f0 = [target_f0]

    return psola.vocode(
        audio, sample_rate=int(sr), target_pitch=corrected_f0, fmin=fmin, fmax=fmax
    )


def shift(target_f0, filepath):
    filepath = Path(filepath)
    y, sr = librosa.load(str(filepath), sr=None, mono=False)

    pitch_corrected_y = shifter(y, sr, target_f0)

    sf.write(str(filepath), pitch_corrected_y, sr)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("target_f0", type=float)
    ap.add_argument("filepath")
    args = ap.parse_args()
    shift(args.target_f0, args.filepath)


if __name__ == "__main__":
    main()
