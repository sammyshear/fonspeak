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
				Wpm:        160,
			},
			{
				Syllable:   "on",
				PitchShift: 261.63,
				Voice:      "he",
				Wpm:        100,
			},
			{
				Syllable:   "ol",
				PitchShift: 293.66,
				Voice:      "he",
				Wpm:        160,
			},
			{
				Syllable:   "@m",
				PitchShift: 329.63,
				Voice:      "he",
				Wpm:        40,
			},
			{
				Syllable:   "aS",
				PitchShift: 349.23,
				Voice:      "he",
				Wpm:        160,
			},
			{
				Syllable:   "eR",
				PitchShift: 392,
				Voice:      "he",
				Wpm:        40,
			},
			{
				Syllable:   "ma",
				PitchShift: 440,
				Voice:      "he",
				Wpm:        160,
			},
			{
				Syllable:   "laX",
				PitchShift: 493.88,
				Voice:      "he",
				Wpm:        50,
			},
		},
		WavFile: f,
	}, 2)
	if err != nil {
		log.Fatal(err)
	}
}
