package fonspeak

import (
	"fmt"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/dsp/window"
	"gonum.org/v1/gonum/floats"
)

const (
	f0DetectionFrameSize = 2048 // Must be a power of 2 for FFT efficiency
)

func estimateFundamentalFrequency(audioData []float64, sampleRate int) (float64, error) {
	if len(audioData) < f0DetectionFrameSize {
		return 0, fmt.Errorf("audio data too short for F0 estimation frame size (%d samples required)", f0DetectionFrameSize)
	}

	// Use a central segment of the audio for F0 estimation to avoid transients at start/end
	startIdx := max(len(audioData)/2-f0DetectionFrameSize/2, 0)
	segment := make([]float64, f0DetectionFrameSize)
	copy(segment, audioData[startIdx:startIdx+f0DetectionFrameSize])

	segment = window.Hann(segment) // Apply window to the segment

	fft := fourier.NewFFT(f0DetectionFrameSize)
	cmplxFFT := fourier.NewCmplxFFT(f0DetectionFrameSize)
	// 1. Compute the single-sided spectrum for a real-valued input.
	// This spectrum contains DC, positive frequencies, and Nyquist (if N is even).
	// Its length is N/2 + 1 (e.g., 1025 for N=2048).
	positiveSpectrum := fft.Coefficients(nil, segment)

	fullPSD := make([]complex128, f0DetectionFrameSize)

	// Copy DC component (index 0)
	fullPSD[0] = positiveSpectrum[0] * cmplx.Conj(positiveSpectrum[0])

	// Copy positive frequencies and create their mirrored negative counterparts.
	// Loop from 1 up to (but not including) the Nyquist frequency.
	// The Nyquist frequency (if N is even) is handled separately as it's its own conjugate.
	for i := 1; i < f0DetectionFrameSize/2; i++ { // For N=2048, i goes from 1 to 1023
		psdVal := positiveSpectrum[i] * cmplx.Conj(positiveSpectrum[i])
		fullPSD[i] = psdVal                      // Positive frequency
		fullPSD[f0DetectionFrameSize-i] = psdVal // Mirrored negative frequency
	}

	// Handle Nyquist frequency (at N/2) if f0DetectionFrameSize is even.
	// For N=2048, Nyquist is at index 1024.
	if f0DetectionFrameSize%2 == 0 {
		nyquistIdx := f0DetectionFrameSize / 2 // 1024 for 2048
		// The Nyquist component is also symmetric and real in the PSD of a real signal.
		fullPSD[nyquistIdx] = positiveSpectrum[nyquistIdx] * cmplx.Conj(positiveSpectrum[nyquistIdx])
	}

	// Compute IFFT of PSD to get Autocorrelation Function (ACF)
	// The ACF reveals periodicity in the signal. A peak in ACF (excluding lag 0)
	// corresponds to the period of the fundamental frequency.
	acfComplex := cmplxFFT.Sequence(nil, fullPSD)
	acf := make([]float64, f0DetectionFrameSize)
	for i := range f0DetectionFrameSize {
		acf[i] = real(acfComplex[i]) // ACF is the real part of the IFFT of PSD
	}

	// Normalize ACF by its value at lag 0 (autocorrelation with itself)
	if acf[0] != 0 {
		floats.Scale(1/acf[0], acf)
	}

	// Find the peak in the ACF to estimate the period (lag).
	// We search within a plausible human vocal/instrumental F0 range.
	// Frequencies outside this range are likely noise or harmonics.
	// Period (in samples) = SampleRate / Frequency (Hz)
	const minF0Hz = 50.0   // Lower bound for F0 search (e.g., typical bass voice)
	const maxF0Hz = 1000.0 // Upper bound for F0 search (e.g., typical soprano or higher instrument)

	minPeriodSamples := int(float64(sampleRate) / maxF0Hz)
	maxPeriodSamples := int(float64(sampleRate) / minF0Hz)

	// Clamp bounds to prevent array out-of-bounds access
	if minPeriodSamples < 1 {
		minPeriodSamples = 1
	}
	if maxPeriodSamples >= f0DetectionFrameSize {
		maxPeriodSamples = f0DetectionFrameSize - 1
	}

	peakVal := 0.0
	peakIdx := -1

	// Iterate through plausible period lags to find the highest peak
	for i := minPeriodSamples; i <= maxPeriodSamples; i++ {
		// We're looking for the first strong peak after the initial peak at lag 0.
		// Higher peaks might represent harmonics, but the first significant peak is often F0.
		// A simple threshold can help filter out very weak "peaks" from noise.
		if acf[i] > peakVal && acf[i] > 0.1 { // Require a correlation value above 0.1
			peakVal = acf[i]
			peakIdx = i
		}
	}

	if peakIdx == -1 {
		return 0, fmt.Errorf("no significant fundamental frequency detected in the range %.1f-%.1f Hz", minF0Hz, maxF0Hz)
	}

	estimatedPeriodSamples := float64(peakIdx)
	estimatedF0 := float64(sampleRate) / estimatedPeriodSamples
	return estimatedF0, nil
}
