package main

import (
	//"fmt"
	"machine"
	"time"

	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
	"machine/usb/hid/keyboard"
	"machine/usb/hid/mouse"
	"tinygo.org/x/drivers/encoders"
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

func main() {
	machine.InitADC()

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
		keyboard.KeyLeftGUI, keyboard.KeyRight, keyboard.KeyLeftAlt,
		keyboard.KeyC, keyboard.KeyV, keyboard.KeyTab,
	}

	ws := NewWS2812B(machine.GPIO1)

	for _, c := range colPins {
		c.Configure(machine.PinConfig{Mode: machine.PinOutput})
		c.Low()
	}

	for _, c := range rowPins {
		c.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}

	colors := []uint32{
		0x00000000, 0x00000000, 0x00000000, 0x00000000,
		0x00000000, 0x00000000, 0x00000000, 0x00000000,
		0x00000000, 0x00000000, 0x00000000, 0x00000000,
	}

	kb := keyboard.Port()
	m := mouse.Port()

	dd := 1
	for {
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
			m.Move(dx, dy)
		}

		curr1 := btn1.Get()
		if prev1 && !curr1 {
			dd = 2
		} else if !prev1 && curr1 {
			dd = 1
		}
		prev1 = curr1

		curr2 := btn2.Get()
		if prev2 && !curr2 {
			m.Press(mouse.Left)
		} else if !prev2 && curr2 {
			m.Release(mouse.Left)
		}
		prev2 = curr2

		newv := enc.Position()
		if newv > oldv {
			m.WheelDown()
		} else if newv < oldv {
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
					colors[i] = 0xFFFFFFFF
					kb.Down(keyc[i])
				} else {
					colors[i] = 0x00000000
					kb.Up(keyc[i])
				}
			}
		}
		oldk = newk

		ws.WriteRaw(colors)

		time.Sleep(5 * time.Millisecond)
	}
}
