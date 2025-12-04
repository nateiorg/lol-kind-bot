package ui

import (
	"log"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// normalizeText fixes special character rendering issues in Fyne labels
func normalizeText(text string) string {
	// Replace em dashes and en dashes with regular dashes
	text = strings.ReplaceAll(text, "\u2013", "-") // En dash
	text = strings.ReplaceAll(text, "\u2014", "-") // Em dash
	text = strings.ReplaceAll(text, "\u2015", "-") // Horizontal bar
	// Replace smart quotes with regular quotes
	text = strings.ReplaceAll(text, "\u201C", `"`) // Left double quotation
	text = strings.ReplaceAll(text, "\u201D", `"`) // Right double quotation
	text = strings.ReplaceAll(text, "\u2018", `'`) // Left single quotation
	text = strings.ReplaceAll(text, "\u2019", `'`) // Right single quotation
	return text
}

// ShowMessagesDialogFyne shows a beautiful Fyne messages dialog
func ShowMessagesDialogFyne(messages []string) {
	if len(messages) == 0 {
		log.Printf("ShowMessagesDialog called with empty messages")
		return
	}

	log.Printf("ShowMessagesDialog called with %d messages", len(messages))

	// Show toast notification
	ShowToast("LoL Kind Bot", "Post-game messages ready!")

	// Get Fyne app and ensure it's ready
	app := GetFyneApp()
	if app == nil {
		log.Printf("Fyne app not available")
		return
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

	// Use fyne.DoAndWait() to create window and widgets (like first-run dialog)
	done := make(chan bool, 1)

	var window fyne.Window
	var messageList *widget.List
	var copyButton *widget.Button
	var closeButton *widget.Button
	var content fyne.CanvasObject

	fyne.DoAndWait(func() {

		// Create window
		window = app.NewWindow("LoL Kind Bot - Message Suggestions")
		window.Resize(fyne.NewSize(700, 550))
		window.CenterOnScreen()
		window.SetFixedSize(false)
		// Create list widget for messages with glass styling
		messageList = widget.NewList(
			func() int {
				return len(messages)
			},
			func() fyne.CanvasObject {
				label := widget.NewLabel("")
				label.Wrapping = fyne.TextWrapWord
				return label
			},
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				label := obj.(*widget.Label)
				// Normalize special characters to prevent rendering issues
				text := normalizeText(messages[id])
				label.SetText(text)
				label.Wrapping = fyne.TextWrapWord
			},
		)

		// Track selected index
		var selectedIndex int = 0
		messageList.OnSelected = func(id widget.ListItemID) {
			selectedIndex = id
		}

		// Copy function
		copyMessage := func() {
			if selectedIndex >= 0 && selectedIndex < len(messages) {
				copyMessageToClipboardFyne(messages[selectedIndex])
			} else if len(messages) > 0 {
				copyMessageToClipboardFyne(messages[0])
			}
		}

		// Copy Selected button
		copyButton = widget.NewButton("Copy Selected", copyMessage)
		copyButton.Importance = widget.MediumImportance

		// Close button
		closeButton = widget.NewButton("Close", func() {
			done <- true
			// Close window directly - we're already in Fyne context
			window.Close()
		})
		closeButton.Importance = widget.HighImportance

		// Beautiful instructions card with glass styling
		instructionsLabel := widget.NewRichTextFromMarkdown("**Double-click** a message to copy it, or select and click **Copy Selected**")
		instructionsLabel.Wrapping = fyne.TextWrapWord
		instructionsCard := widget.NewCard("", "", container.NewPadded(instructionsLabel))

		// Beautiful button bar with glass background
		buttonBar := container.NewBorder(
			nil, nil,
			nil,
			container.NewHBox(
				container.NewPadded(widget.NewLabel("")), // Spacer
				container.NewPadded(copyButton),
				container.NewPadded(closeButton),
			),
		)

		// Create a glass-styled container for the message list
		messageContainer := container.NewPadded(
			container.NewBorder(
				nil, nil, nil, nil,
				messageList,
			),
		)

		// Layout with glass morphism effects
		content = container.NewBorder(
			container.NewPadded(instructionsCard),
			buttonBar,
			nil,
			nil,
			messageContainer,
		)

		// Apply Windows glass effect (Mica/Acrylic blur)
		ApplyGlassEffect(window)

		// Set content directly - effects can cause rendering issues
		window.SetContent(content)

		// Select first item
		if len(messages) > 0 {
			messageList.Select(0)
		}

		window.SetCloseIntercept(func() {
			done <- true
			// Don't call window.Close() here - it's already closing
		})

		// Show window and bring to front
		// app.Run() is running in a goroutine, so Show() will work properly
		window.Show()
		window.RequestFocus()
		window.CenterOnScreen()
	})

	// Wait for window to close
	<-done
}

// Wrapper function to maintain compatibility
func ShowMessagesDialog(messages []string) {
	ShowMessagesDialogFyne(messages)
}

// copyMessageToClipboardFyne copies a message to clipboard and shows feedback
func copyMessageToClipboardFyne(message string) {
	if err := clipboard.WriteAll(message); err != nil {
		log.Printf("Failed to copy to clipboard: %v", err)
		ShowToast("LoL Kind Bot", "Failed to copy message")
	} else {
		log.Printf("Copied to clipboard: %s", message)
		ShowToast("LoL Kind Bot", "Message copied to clipboard!")
	}
}
