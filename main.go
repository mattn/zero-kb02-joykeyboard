package main

import (
	//"fmt"
	"image/color"
	"machine"
	"math"
	"time"

	"machine/usb/hid/keyboard"
	"machine/usb/hid/mouse"

	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"

	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/encoders"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/gophers"
)

type WS2812B struct {
	Pin machine.Pin
	ws  *piolib.WS2812B
}

func NewWS2812B(pin machine.Pin) *WS2812B {
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, pin)
	ws.EnableDMA(true)
	return &WS2812B{
		ws: ws,
	}
}

func (ws *WS2812B) WriteRaw(rawGRB []uint32) error {
	return ws.ws.WriteRaw(rawGRB)
}

type RGB struct {
	R, G, B uint8
}

func min3(a, b, c float64) float64 {
	return math.Min(math.Min(a, b), c)
}

func hueToRGB(h, s, v float64) RGB {
	h = math.Mod(h, 360)
	h /= 60
	i := math.Floor(h)
	f := h - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}

	return RGB{
		R: uint8(math.Round(r * 255)),
		G: uint8(math.Round(g * 255)),
		B: uint8(math.Round(b * 255)),
	}
}

func main() {
	machine.I2C0.Configure(machine.I2CConfig{
		Frequency: 2.8 * machine.MHz,
		SDA:       machine.GPIO12,
		SCL:       machine.GPIO13,
	})

	machine.InitADC()

	display := ssd1306.NewI2C(machine.I2C0)
	display.Configure(ssd1306.Config{
		Address: 0x3C,
		Width:   128,
		Height:  64,
	})
	display.SetRotation(drivers.Rotation180)

	ax := machine.ADC{Pin: machine.GPIO29}
	ax.Configure(machine.ADCConfig{})
	ay := machine.ADC{Pin: machine.GPIO28}
	ay.Configure(machine.ADCConfig{})
	ox, oy := int(ax.Get()), int(ay.Get())

	btn1 := machine.GPIO0
	btn1.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	prev1 := btn1.Get()

	btn2 := machine.GPIO2
	btn2.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	prev2 := btn2.Get()

	enc := encoders.NewQuadratureViaInterrupt(
		machine.GPIO3,
		machine.GPIO4,
	)
	enc.Configure(encoders.QuadratureConfig{
		Precision: 4,
	})
	oldv := enc.Position()

	colPins := []machine.Pin{
		machine.GPIO5,
		machine.GPIO6,
		machine.GPIO7,
		machine.GPIO8,
	}

	rowPins := []machine.Pin{
		machine.GPIO9,
		machine.GPIO10,
		machine.GPIO11,
	}

	var newk [12]bool
	var oldk [12]bool
	keyc := [12]keyboard.Keycode{
		keyboard.KeyLeftShift, keyboard.KeyLeft, keyboard.KeyLeftCtrl,
		keyboard.KeyUp, keyboard.KeyMenu, keyboard.KeyDown,
		keyboard.KeyTab, keyboard.KeyRight, keyboard.KeyLeftAlt,
		keyboard.KeyC, keyboard.KeyV, keyboard.KeyEnter,
	}

	ws := NewWS2812B(machine.GPIO1)

	for _, c := range colPins {
		c.Configure(machine.PinConfig{Mode: machine.PinOutput})
		c.Low()
	}

	for _, c := range rowPins {
		c.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}

	colors := [12]int{}
	btnc := [12]uint32{}

	kb := keyboard.Port()
	m := mouse.Port()

	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	data := []byte("ABCEF")

	ci := 0
	var updated bool
	update := 0

	dd := 1
	for {
		updated = false

		x := int(ax.Get())
		y := int(ay.Get())
		dx := 0
		if x-ox > 0x1000 {
			dx = dd
		} else if x-ox < -0x1000 {
			dx = -dd
		}
		dy := 0
		if y-oy > 0x1000 {
			dy = -dd
		} else if y-oy < -0x1000 {
			dy = dd
		}
		if dx != 0 || dy != 0 {
			if update == 0 {
				update = 20
			}
			updated = true
			m.Move(dx, dy)
		} else {
			update = 0
		}

		curr1 := btn1.Get()
		if prev1 && !curr1 {
			updated = true
			dd = 2
		} else if !prev1 && curr1 {
			updated = true
			dd = 1
		}
		prev1 = curr1

		curr2 := btn2.Get()
		if prev2 && !curr2 {
			updated = true
			m.Press(mouse.Left)
		} else if !prev2 && curr2 {
			updated = true
			m.Release(mouse.Left)
		}
		prev2 = curr2

		newv := enc.Position()
		if newv > oldv {
			updated = true
			m.WheelDown()
		} else if newv < oldv {
			updated = true
			m.WheelUp()
		}
		oldv = newv

		// COL1
		colPins[0].High()
		colPins[1].Low()
		colPins[2].Low()
		colPins[3].Low()
		time.Sleep(1 * time.Millisecond)
		newk[0] = rowPins[0].Get()
		newk[1] = rowPins[1].Get()
		newk[2] = rowPins[2].Get()
		// COL2
		colPins[0].Low()
		colPins[1].High()
		colPins[2].Low()
		colPins[3].Low()
		time.Sleep(1 * time.Millisecond)
		newk[3] = rowPins[0].Get()
		newk[4] = rowPins[1].Get()
		newk[5] = rowPins[2].Get()
		// COL3
		colPins[0].Low()
		colPins[1].Low()
		colPins[2].High()
		colPins[3].Low()
		time.Sleep(1 * time.Millisecond)
		newk[6] = rowPins[0].Get()
		newk[7] = rowPins[1].Get()
		newk[8] = rowPins[2].Get()
		// COL4
		colPins[0].Low()
		colPins[1].Low()
		colPins[2].Low()
		colPins[3].High()
		time.Sleep(1 * time.Millisecond)
		newk[9] = rowPins[0].Get()
		newk[10] = rowPins[1].Get()
		newk[11] = rowPins[2].Get()

		for i, k := range newk {
			if newk[i] != oldk[i] {
				if k {
					updated = true
					colors[i] = 100
					ci += 30
					kb.Down(keyc[i])
				} else {
					kb.Up(keyc[i])
				}
			} else {
				if colors[i] > 0 {
					colors[i] -= 5
					rgb := hueToRGB(float64(ci), 1.0, float64(colors[i])/100)
					btnc[i] = uint32(rgb.G)<<24 | uint32(rgb.R)<<16 | uint32(rgb.B)<<8 | uint32(0xFF)
				} else {
					btnc[i] = 0
				}
			}
		}
		oldk = newk

		ws.WriteRaw(btnc[:])

		if updated {
			if update > 0 {
				update--
			}
			if update == 0 {
				data[0], data[1], data[2], data[3], data[4] = data[1], data[2], data[3], data[4], data[0]
				display.ClearDisplay()
				tinyfont.WriteLine(&display, &gophers.Regular32pt, 5, 45, string(data), white)
				display.Display()
			}
		}

		time.Sleep(5 * time.Millisecond)
	}
}
