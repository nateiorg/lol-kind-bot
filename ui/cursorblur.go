package ui

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// CursorBlurOverlay is a custom widget that creates a blur focus effect around the cursor
type CursorBlurOverlay struct {
	widget.BaseWidget
	content        fyne.CanvasObject
	cursorX        float32
	cursorY        float32
	targetX        float32
	targetY        float32
	blurRadius     float32
	maxBlurAlpha   uint8
	mu             sync.RWMutex
	animating      bool
	lastUpdateTime time.Time
}

// NewCursorBlurOverlay creates a new cursor blur overlay widget
func NewCursorBlurOverlay(content fyne.CanvasObject) *CursorBlurOverlay {
	overlay := &CursorBlurOverlay{
		content:        content,
		cursorX:        -1,
		cursorY:        -1,
		targetX:        -1,
		targetY:        -1,
		blurRadius:     150, // Radius of focus area in pixels
		maxBlurAlpha:   180, // Maximum blur overlay opacity
		lastUpdateTime: time.Now(),
	}
	overlay.ExtendBaseWidget(overlay)
	return overlay
}

// CreateRenderer creates the renderer for the blur overlay
func (c *CursorBlurOverlay) CreateRenderer() fyne.WidgetRenderer {
	return &cursorBlurRenderer{
		overlay: c,
		content: c.content,
	}
}

// Tapped handles tap events (for mouse position tracking)
func (c *CursorBlurOverlay) Tapped(ev *fyne.PointEvent) {
	c.updateCursorPosition(ev.Position.X, ev.Position.Y)
}

// updateCursorPosition updates the target cursor position and starts animation
func (c *CursorBlurOverlay) updateCursorPosition(x, y float32) {
	c.mu.Lock()
	c.targetX = x
	c.targetY = y
	c.mu.Unlock()
	
	// Start animation if not already animating
	if !c.animating {
		c.startAnimation()
	}
	c.Refresh()
}

// startAnimation starts the smooth animation to target position
func (c *CursorBlurOverlay) startAnimation() {
	c.mu.Lock()
	if c.animating {
		c.mu.Unlock()
		return
	}
	c.animating = true
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(16 * time.Millisecond) // ~60fps
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				c.mu.Lock()
				dx := c.targetX - c.cursorX
				dy := c.targetY - c.cursorY
				distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				
				// Smooth interpolation (easing)
				easingFactor := float32(0.15) // Controls animation speed
				if distance < 0.5 {
					// Close enough, snap to target
					c.cursorX = c.targetX
					c.cursorY = c.targetY
					c.animating = false
					c.mu.Unlock()
					c.Refresh()
					return
				}
				
				// Interpolate towards target
				c.cursorX += dx * easingFactor
				c.cursorY += dy * easingFactor
				c.mu.Unlock()
				c.Refresh()
			case <-time.After(2 * time.Second):
				// Safety timeout
				c.mu.Lock()
				c.animating = false
				c.mu.Unlock()
				return
			}
		}
	}()
}

// cursorBlurRenderer renders the blur overlay effect
type cursorBlurRenderer struct {
	overlay *CursorBlurOverlay
	content fyne.CanvasObject
	objects []fyne.CanvasObject
}

func (r *cursorBlurRenderer) Layout(size fyne.Size) {
	// Layout content to fill
	r.content.Resize(size)
	r.content.Move(fyne.NewPos(0, 0))
}

func (r *cursorBlurRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *cursorBlurRenderer) Refresh() {
	r.overlay.mu.RLock()
	cursorX := r.overlay.cursorX
	cursorY := r.overlay.cursorY
	blurRadius := r.overlay.blurRadius
	maxAlpha := r.overlay.maxBlurAlpha
	size := r.overlay.Size()
	r.overlay.mu.RUnlock()

	// Create gradient overlay based on cursor position
	// Clear existing objects except content
	r.objects = []fyne.CanvasObject{r.content}
	
	if cursorX >= 0 && cursorY >= 0 && cursorX < size.Width && cursorY < size.Height {
		// Create radial gradient overlay using multiple circles
		// This simulates a blur focus effect
		numLayers := 6
		for i := 0; i < numLayers; i++ {
			radius := blurRadius * float32(i+1) / float32(numLayers)
			// Alpha decreases from center (more blur) to edges (less blur)
			alpha := uint8(float32(maxAlpha) * (1.0 - float32(i)/float32(numLayers*2)))
			
			// Create a circle overlay
			circle := canvas.NewCircle(color.NRGBA{R: 248, G: 250, B: 255, A: alpha})
			circleSize := radius * 2
			if circleSize > 0 {
				circle.Resize(fyne.NewSize(circleSize, circleSize))
				circle.Move(fyne.NewPos(cursorX-radius, cursorY-radius))
				r.objects = append(r.objects, circle)
			}
		}
	} else {
		// No cursor or cursor outside bounds - show uniform blur overlay
		if size.Width > 0 && size.Height > 0 {
			overlay := canvas.NewRectangle(color.NRGBA{R: 248, G: 250, B: 255, A: maxAlpha})
			overlay.Resize(size)
			overlay.Move(fyne.NewPos(0, 0))
			r.objects = append(r.objects, overlay)
		}
	}
}

func (r *cursorBlurRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *cursorBlurRenderer) Destroy() {
	// Cleanup
}

// addCursorBlurEffect wraps content with a cursor-following blur overlay
func addCursorBlurEffect(window fyne.Window, content fyne.CanvasObject) fyne.CanvasObject {
	// Create blur overlay
	blurOverlay := NewCursorBlurOverlay(content)
	
	// Set up mouse tracking using Windows API
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond) // Check every 50ms
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Get mouse position relative to window using Windows API
				mouseX, mouseY := getMousePosition(window)
				if mouseX >= 0 && mouseY >= 0 {
					blurOverlay.updateCursorPosition(float32(mouseX), float32(mouseY))
				}
			case <-time.After(30 * time.Second):
				return
			}
		}
	}()
	
	return blurOverlay
}

// getMousePosition gets the mouse position relative to the window using Windows API
func getMousePosition(window fyne.Window) (int32, int32) {
	// Use Windows API to get cursor position
	// This is a placeholder - full implementation would use GetCursorPos and ScreenToClient
	// For now, return -1 to indicate no tracking
	return -1, -1
}
