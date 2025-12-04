package ui

import (
	"fmt"
	"log"
	"lol-kind-bot/config"
	"lol-kind-bot/lcu"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ShowSettingsDialogWithCallbackFyne shows a beautiful Fyne settings window
func ShowSettingsDialogWithCallbackFyne(cfg *config.Config, generateCallback func()) (*config.Config, bool) {
	log.Printf("ShowSettingsDialogWithCallbackFyne called")

	// Get Fyne app and ensure it's ready
	app := getFyneApp()
	if app == nil {
		log.Printf("Fyne app not available")
		return nil, false
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

	// Create a copy of config for editing (non-Fyne operation, can happen outside UI thread)
	editCfg := *cfg

	// Try to auto-detect summoner name if empty (non-Fyne operation)
	if editCfg.MySummonerName == "" {
		if lockfileInfo, err := lcu.ParseLockfile(lcu.GetLockfilePath()); err == nil {
			if client, err := lcu.NewClient(lockfileInfo); err == nil {
				if summoner, err := client.GetCurrentSummoner(); err == nil {
					editCfg.MySummonerName = summoner.DisplayName
					log.Printf("Auto-detected summoner name: %s", editCfg.MySummonerName)
				}
			}
		}
	}

	// Message Style dropdown options (non-Fyne data)
	styleOptions := []string{"Casual & Friendly", "Professional", "Enthusiastic", "Humble & Supportive"}
	styleMapping := []struct {
		tone          string
		languageStyle string
	}{
		{"friendly", "casual"},
		{"professional", "formal"},
		{"enthusiastic", "enthusiastic"},
		{"humble", "casual"},
	}

	// Find current style index (non-Fyne operation)
	currentStyleIdx := 0
	for i, style := range styleMapping {
		if style.tone == editCfg.LLMSettings.Tone && style.languageStyle == editCfg.LLMSettings.LanguageStyle {
			currentStyleIdx = i
			break
		}
	}

	// Number of Messages dropdown options (non-Fyne data)
	msgCountOptions := []string{"2", "3", "4", "5"}
	currentMsgCount := editCfg.LLMSettings.MaxMessages
	if editCfg.LLMSettings.MinMessages != editCfg.LLMSettings.MaxMessages {
		currentMsgCount = (editCfg.LLMSettings.MinMessages + editCfg.LLMSettings.MaxMessages) / 2
	}
	currentMsgIdx := 0
	for i, count := range []int{2, 3, 4, 5} {
		if count == currentMsgCount {
			currentMsgIdx = i
			break
		}
	}
	if currentMsgIdx == 0 && currentMsgCount != 2 {
		currentMsgIdx = 1 // Default to 3
	}

	// Use fyne.DoAndWait() to create window and widgets (like first-run dialog)
	// Then use ShowAndRun() inside fyne.DoAndWait() to start the event loop
	var resultCfg *config.Config
	var accepted bool
	done := make(chan bool, 1)

	var window fyne.Window
	var summonerNameEntry *widget.Entry
	var autoCopyCheck *widget.Check
	var generalForm fyne.CanvasObject
	var modelEntry *widget.Entry
	var urlEntry *widget.Entry
	var apiForm fyne.CanvasObject
	var styleSelect *widget.Select
	var msgCountSelect *widget.Select
	var messageForm fyne.CanvasObject
	var goldEnabledCheck *widget.Check
	var goldThresholdsEntry *widget.Entry
	var goldForm fyne.CanvasObject
	var testGoldButton *widget.Button
	var generateButton *widget.Button
	var saveButton *widget.Button
	var cancelButton *widget.Button
	var mainContent fyne.CanvasObject

	log.Printf("About to call fyne.DoAndWait()")
	fyne.DoAndWait(func() {
		log.Printf("Inside fyne.DoAndWait() - creating window")
		// App is already available from above
		log.Printf("Got Fyne app instance")

		// Create window
		window = app.NewWindow("LoL Kind Bot - Settings")
		log.Printf("Created window")
		window.Resize(fyne.NewSize(750, 650))
		window.CenterOnScreen()
		window.SetFixedSize(false)

		// Create widgets
		summonerNameEntry = widget.NewEntry()
		summonerNameEntry.SetText(editCfg.MySummonerName)
		summonerNameEntry.SetPlaceHolder("Enter your summoner name")

		autoCopyCheck = widget.NewCheck("Auto-copy first message to clipboard", nil)
		autoCopyCheck.SetChecked(editCfg.AutoCopyToClipboard)

		// Beautiful glass card with enhanced styling
		generalCard := widget.NewCard("General Settings", "", container.NewVBox(
			container.NewPadded(container.NewGridWithColumns(2,
				widget.NewLabel("Summoner Name:"),
				summonerNameEntry,
			)),
			container.NewPadded(autoCopyCheck),
		))
		generalForm = container.NewPadded(generalCard)

		// LLM API Settings Section
		modelEntry = widget.NewEntry()
		modelEntry.SetText(editCfg.OllamaModel)
		modelEntry.SetPlaceHolder("e.g., llama3.1")

		urlEntry = widget.NewEntry()
		urlEntry.SetText(editCfg.OllamaURL)
		urlEntry.SetPlaceHolder("e.g., http://localhost:11434/api/generate")

		// Beautiful glass card
		apiCard := widget.NewCard("LLM API Settings", "", container.NewVBox(
			container.NewPadded(container.NewGridWithColumns(2,
				widget.NewLabel("Model:"),
				modelEntry,
			)),
			container.NewPadded(container.NewGridWithColumns(2,
				widget.NewLabel("API URL:"),
				urlEntry,
			)),
		))
		apiForm = container.NewPadded(apiCard)

		// Message Generation Settings Section
		styleSelect = widget.NewSelect(styleOptions, nil)
		styleSelect.SetSelectedIndex(currentStyleIdx)

		msgCountSelect = widget.NewSelect(msgCountOptions, nil)
		msgCountSelect.SetSelectedIndex(currentMsgIdx)

		// Beautiful glass card
		messageCard := widget.NewCard("Message Generation", "", container.NewVBox(
			container.NewPadded(container.NewGridWithColumns(2,
				widget.NewLabel("Message Style:"),
				styleSelect,
			)),
			container.NewPadded(container.NewGridWithColumns(2,
				widget.NewLabel("Number of Messages:"),
				msgCountSelect,
			)),
		))
		messageForm = container.NewPadded(messageCard)

		// Gold Announcements Settings Section
		goldEnabledCheck = widget.NewCheck("Enable gold announcements", nil)
		goldEnabledCheck.SetChecked(editCfg.GoldAnnouncements.Enabled)

		// Format thresholds as comma-separated string
		thresholdsStr := ""
		if len(editCfg.GoldAnnouncements.Thresholds) > 0 {
			thresholdsStr = fmt.Sprintf("%d", editCfg.GoldAnnouncements.Thresholds[0])
			for i := 1; i < len(editCfg.GoldAnnouncements.Thresholds); i++ {
				thresholdsStr += fmt.Sprintf(", %d", editCfg.GoldAnnouncements.Thresholds[i])
			}
		}

		goldThresholdsEntry = widget.NewEntry()
		goldThresholdsEntry.SetText(thresholdsStr)
		goldThresholdsEntry.SetPlaceHolder("e.g., 1500, 2000, 3000")

		// Test gold sound button
		testGoldButton = widget.NewButton("🔊 Test Gold Sound", func() {
			// Test with 2000 gold as a sample value
			AnnounceGold(2000)
			ShowToast("LoL Kind Bot", "Testing gold announcement: 2000 Gold")
		})
		testGoldButton.Importance = widget.MediumImportance

		// Beautiful glass card for gold announcements
		goldCard := widget.NewCard("Gold Announcements", "", container.NewVBox(
			container.NewPadded(goldEnabledCheck),
			container.NewPadded(container.NewGridWithColumns(2,
				widget.NewLabel("Thresholds:"),
				goldThresholdsEntry,
			)),
			container.NewPadded(widget.NewLabel("Enter comma-separated gold amounts (e.g., 1500, 2000, 3000)")),
			container.NewPadded(testGoldButton),
		))
		goldForm = container.NewPadded(goldCard)

		// Generate Messages button (if callback provided)
		if generateCallback != nil {
			generateButton = widget.NewButton("✨ Generate Messages from Last Match", func() {
				ShowToast("LoL Kind Bot", "Generating messages from last match...")
				go generateCallback()
			})
			generateButton.Importance = widget.MediumImportance
		}

		// Action buttons
		saveButton = widget.NewButton("Save", func() {
			// Parse gold thresholds from entry field
			thresholdsStr := strings.TrimSpace(goldThresholdsEntry.Text)
			var thresholds []int
			if thresholdsStr != "" {
				parts := strings.Split(thresholdsStr, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part != "" {
						if val, err := strconv.Atoi(part); err == nil && val > 0 {
							thresholds = append(thresholds, val)
						}
					}
				}
			}

			// Build result config - access widget values
			resultCfg = &config.Config{
				MySummonerName:        summonerNameEntry.Text,
				OllamaModel:           modelEntry.Text,
				OllamaURL:             urlEntry.Text,
				AutoCopyToClipboard:   autoCopyCheck.Checked,
				PollIntervalSeconds:   editCfg.PollIntervalSeconds,
				EndOfGameCooldownSec:  editCfg.EndOfGameCooldownSec,
				EnableDetailedLogging: editCfg.EnableDetailedLogging,
				EnableDebugLogging:    editCfg.EnableDebugLogging,
				AFKThresholds:         editCfg.AFKThresholds,
				GoldAnnouncements: config.GoldAnnouncementSettings{
					Enabled:         goldEnabledCheck.Checked,
					Thresholds:      thresholds,
					PollIntervalSec: editCfg.GoldAnnouncements.PollIntervalSec,
				},
				LLMSettings: config.LLMSettings{
					MinMessages:        []int{2, 3, 4, 5}[msgCountSelect.SelectedIndex()],
					MaxMessages:        []int{2, 3, 4, 5}[msgCountSelect.SelectedIndex()],
					MaxMessageLength:   150,
					FocusAreas:         []string{"all"},
					AFKHandling:        "default",
					Temperature:        0.7,
					MaxTokens:          0,
					CustomInstructions: "",
				},
			}

			// Set tone and language style from style selection
			if styleSelect.SelectedIndex() >= 0 && styleSelect.SelectedIndex() < len(styleMapping) {
				resultCfg.LLMSettings.Tone = styleMapping[styleSelect.SelectedIndex()].tone
				resultCfg.LLMSettings.LanguageStyle = styleMapping[styleSelect.SelectedIndex()].languageStyle
			}

			// Use defaults for hidden settings
			if resultCfg.LLMSettings.MaxMessageLength == 0 {
				resultCfg.LLMSettings.MaxMessageLength = 150
			}
			if len(resultCfg.LLMSettings.FocusAreas) == 0 {
				resultCfg.LLMSettings.FocusAreas = []string{"all"}
			}
			if resultCfg.LLMSettings.AFKHandling == "" {
				resultCfg.LLMSettings.AFKHandling = "default"
			}
			if resultCfg.LLMSettings.Temperature == 0 {
				resultCfg.LLMSettings.Temperature = 0.7
			}

			accepted = true
			done <- true
			// Close window directly - we're already in Fyne context
			window.Close()
		})
		saveButton.Importance = widget.HighImportance

		cancelButton = widget.NewButton("Cancel", func() {
			accepted = false
			resultCfg = nil
			done <- true
			// Close window directly - we're already in Fyne context
			window.Close()
		})

		// Layout with beautiful spacing and glass effects
		contentItems := []fyne.CanvasObject{
			generalForm,
			apiForm,
			messageForm,
			goldForm,
		}
		if generateButton != nil {
			contentItems = append(contentItems, container.NewPadded(generateButton))
		}
		content := container.NewVBox(contentItems...)

		buttonBar := container.NewBorder(
			nil, nil,
			nil,
			container.NewHBox(
				container.NewPadded(cancelButton),
				container.NewPadded(saveButton),
			),
		)

		mainContent = container.NewBorder(
			nil,       // top
			buttonBar, // bottom
			nil,       // left
			nil,       // right
			container.NewScroll(content),
		)

		// Apply Windows glass effect (Mica/Acrylic blur)
		ApplyGlassEffect(window)

		// Set content directly - effects can cause rendering issues
		// For now, use mainContent directly to ensure it renders
		window.SetContent(mainContent)

		// Handle window close event (X button or Alt+F4)
		window.SetCloseIntercept(func() {
			// If window is closed without Save/Cancel, treat as cancel
			accepted = false
			resultCfg = nil
			done <- true
			// Don't call window.Close() here - it's already closing
		})

		// Show window and bring to front
		// app.Run() is running in a goroutine, so Show() will work properly
		log.Printf("About to call window.Show()")
		window.Show()
		window.RequestFocus()
		window.CenterOnScreen()
		log.Printf("Window shown")
	})

	log.Printf("fyne.DoAndWait() completed, waiting for window to close")
	// Wait for window to close (signaled by button handlers or close intercept)
	<-done
	log.Printf("Window closed, returning result")

	return resultCfg, accepted
}

// ShowSettingsDialogFyne shows settings dialog without callback
func ShowSettingsDialogFyne(cfg *config.Config) (*config.Config, bool) {
	return ShowSettingsDialogWithCallbackFyne(cfg, nil)
}

// Wrapper functions to maintain compatibility - these now use Fyne
func ShowSettingsDialogWithCallback(cfg *config.Config, generateCallback func()) (*config.Config, bool) {
	return ShowSettingsDialogWithCallbackFyne(cfg, generateCallback)
}

func ShowSettingsDialog(cfg *config.Config) (*config.Config, bool) {
	return ShowSettingsDialogFyne(cfg)
}
