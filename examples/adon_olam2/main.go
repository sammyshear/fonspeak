package main

import (
	"log"
	"math/rand"
	"os"

	"github.com/sammyshear/fonspeak"
)

var syls = []string{"a", "don", "o", "l", "@", "@", "@", "@", "m", "aS", "er", "m", "a", "a", "a", "a", "a", "a", "l", "a", "a", "a", "a", "a", "a", "X"}

func main() {
	f, err := os.Create("adon_olam2.wav")
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	syllables := []fonspeak.Params{}

	for _, syl := range syls {
		syllables = append(syllables, fonspeak.Params{
			Syllable:   syl,
			PitchShift: rand.Float64() * 500,
			Voice:      "he",
			Wpm:        160,
		})
	}

	err = fonspeak.FonspeakPhrase(fonspeak.PhraseParams{
		Syllables: syllables,
		WavFile:   f,
	}, 4)
	if err != nil {
		panic(err)
	}
}
