package ui

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// Speak announces text using Windows SAPI text-to-speech via PowerShell
func Speak(text string) error {
	// Escape single quotes in text for PowerShell
	escapedText := strings.ReplaceAll(text, "'", "''")
	
	// Use PowerShell's built-in text-to-speech
	// This is simpler and more reliable than COM interop
	psCommand := fmt.Sprintf("Add-Type -AssemblyName System.Speech; $speak = New-Object System.Speech.Synthesis.SpeechSynthesizer; $speak.Speak('%s')", escapedText)
	
	log.Printf("TTS: Speaking '%s'", text)
	cmd := exec.Command("powershell", "-Command", psCommand)
	
	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		log.Printf("TTS Error: %v, stdout: %s, stderr: %s", err, stdout.String(), stderr.String())
		return fmt.Errorf("failed to speak: %w (stdout: %s, stderr: %s)", err, stdout.String(), stderr.String())
	}
	
	log.Printf("TTS: Successfully announced '%s'", text)
	return nil
}

// AnnounceGold announces a gold milestone using text-to-speech
func AnnounceGold(gold int) {
	text := fmt.Sprintf("%d Gold", gold)
	log.Printf("AnnounceGold called with %d gold", gold)
	if err := Speak(text); err != nil {
		log.Printf("Failed to announce gold: %v", err)
	} else {
		log.Printf("Successfully announced: %s", text)
	}
}
