package main

import (
	"log"
	"os"

	"github.com/sammyshear/fonspeak"
)

func main() {
	f, err := os.Create("adon_olam.wav")
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	err = fonspeak.FonspeakPhrase(fonspeak.PhraseParams{
		Syllables: []fonspeak.Params{
			{
				Syllable:   "ad",
				PitchShift: 261.63,
				Voice:      "he",
			},
			{
				Syllable:   "on",
				PitchShift: 261.63,
				Voice:      "he",
			},
			{
				Syllable:   "ol",
				PitchShift: 293.66,
				Voice:      "he",
			},
			{
				Syllable:   "@m",
				PitchShift: 329.63,
				Voice:      "he",
			},
			{
				Syllable:   "aS",
				PitchShift: 349.23,
				Voice:      "he",
			},
			{
				Syllable:   "eR",
				PitchShift: 392,
				Voice:      "he",
			},
			{
				Syllable:   "ma",
				PitchShift: 440,
				Voice:      "he",
			},
			{
				Syllable:   "laX",
				PitchShift: 493.88,
				Voice:      "he",
			},
		},
		WavFile: f,
	}, 2)
	if err != nil {
		log.Fatal(err)
	}
}
