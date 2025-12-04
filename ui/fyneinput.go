package ui

import (
	"log"
	"lol-kind-bot/lcu"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ShowFirstRunDialogFyne shows a beautiful Fyne first-run input dialog
func ShowFirstRunDialogFyne() (string, bool) {
	// Try to get summoner name from LCU API if available
	defaultSummonerName := ""
	if lockfileInfo, err := lcu.ParseLockfile(lcu.GetLockfilePath()); err == nil {
		if client, err := lcu.NewClient(lockfileInfo); err == nil {
			if summoner, err := client.GetCurrentSummoner(); err == nil {
				defaultSummonerName = summoner.DisplayName
				log.Printf("Auto-detected summoner name: %s", defaultSummonerName)
			}
		}
	}

	app := getFyneApp()
	if app == nil {
		return "", false
	}
	
	// Wait for app to be ready before creating windows
	if !isAppReady() {
		select {
		case <-appReady:
			// App is ready
		case <-time.After(3 * time.Second):
			// Timeout - proceed anyway
		}
	}
	
	var window fyne.Window
	var entry *widget.Entry
	var continueButton *widget.Button
	var cancelButton *widget.Button
	var content fyne.CanvasObject
	var result string
	var accepted bool
	done := make(chan bool, 1)
	
	// All Fyne UI operations must be wrapped in fyne.DoAndWait() when called from background goroutines
	fyne.DoAndWait(func() {
		window = app.NewWindow("LoL Kind Bot - First Run Setup")
		window.Resize(fyne.NewSize(550, 300))
		window.CenterOnScreen()
		window.SetFixedSize(false)

		// Beautiful welcome card
		welcomeLabel := widget.NewRichTextFromMarkdown("# Welcome to LoL Kind Bot!")
		welcomeLabel.Wrapping = fyne.TextWrapWord
		welcomeCard := widget.NewCard("", "", container.NewPadded(welcomeLabel))

		// Instruction label
		instructionLabel := widget.NewLabel("Please enter your summoner name to get started:")
		instructionLabel.Wrapping = fyne.TextWrapWord

		// Input entry
		entry = widget.NewEntry()
		entry.SetPlaceHolder("Enter your summoner name")
		if defaultSummonerName != "" {
			entry.SetText(defaultSummonerName)
		}
		entry.OnSubmitted = func(text string) {
			text = strings.TrimSpace(text)
			if text != "" {
				result = text
				accepted = true
				done <- true
				// Close window directly - we're already in Fyne context
				window.Close()
			}
		}

		// Buttons
		continueButton = widget.NewButton("Continue", func() {
			text := strings.TrimSpace(entry.Text)
			if text == "" && defaultSummonerName != "" {
				text = defaultSummonerName
			}
			if text != "" {
				result = text
				accepted = true
				done <- true
				// Close window directly - we're already in Fyne context
				window.Close()
			}
		})
		continueButton.Importance = widget.HighImportance

		cancelButton = widget.NewButton("Cancel", func() {
			accepted = false
			result = ""
			done <- true
			// Close window directly - we're already in Fyne context
			window.Close()
		})

		// Beautiful layout with glass effects
		content = container.NewVBox(
			welcomeCard,
			widget.NewSeparator(),
			container.NewPadded(instructionLabel),
			container.NewPadded(entry),
			widget.NewSeparator(),
			container.NewBorder(
				nil, nil,
				nil,
				container.NewHBox(
					container.NewPadded(widget.NewLabel("")), // Spacer
					container.NewPadded(cancelButton),
					container.NewPadded(continueButton),
				),
			),
		)

		// Apply Windows glass effect (Mica/Acrylic blur)
		ApplyGlassEffect(window)
		
		// Set content directly - effects can cause rendering issues
		paddedContent := container.NewPadded(content)
		window.SetContent(paddedContent)

		// Focus on entry
		entry.FocusGained()

		window.SetCloseIntercept(func() {
			if result == "" {
				accepted = false
			}
			done <- true
			// Don't call window.Close() here - it's already closing
		})

		// Show window and bring to front
		window.Show()
		window.RequestFocus()
		window.CenterOnScreen()
	})
	
	<-done

	return result, accepted && result != ""
}

// Wrapper function to maintain compatibility
func ShowFirstRunDialog() (string, bool) {
	return ShowFirstRunDialogFyne()
}
