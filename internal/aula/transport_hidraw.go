//go:build linux

package aula

import (
	"fmt"
	"time"

	"github.com/ldgnu/openaula/internal/hidraw"
)

// hidrawTransport talks to the wired keyboard through /dev/hidraw nodes.
// Feature reports go to the configuration interface (interface 3) and LCD
// pixel pages to the LCD interface (interface 2).
type hidrawTransport struct {
	feature *hidraw.Device
	lcd     *hidraw.Device
}

func openTransport() (Transport, error) {
	return openTransportFor(VendorID, ProductID)
}

// openTransportFor opens the configuration interface of a device.
func openTransportFor(vid, pid uint16) (Transport, error) {
	infos, err := hidraw.Enumerate()
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if info.VendorID != vid || info.ProductID != pid {
			continue
		}
		if info.Interface != InterfaceID {
			continue
		}
		dev, err := hidraw.Open(info.Path)
		if err != nil {
			return nil, fmt.Errorf("abriendo %s: %w", info.Path, err)
		}
		return &hidrawTransport{feature: dev}, nil
	}
	return nil, fmt.Errorf("teclado no encontrado (VID=%04x PID=%04x)", vid, pid)
}

func (t *hidrawTransport) SetFeatureReport(data [reportSize]byte) error {
	// Devices without report IDs take the 64-byte payload as is; some hidraw
	// stacks expect the 0x00 report-ID prefix, so try that as a fallback.
	if err := t.feature.SetFeature(data[:]); err == nil {
		return nil
	}
	buf := append([]byte{0x00}, data[:]...)
	if err := t.feature.SetFeature(buf); err != nil {
		return fmt.Errorf("SET_FEATURE: %w", err)
	}
	return nil
}

func (t *hidrawTransport) GetFeatureReport() ([reportSize]byte, error) {
	var out [reportSize]byte
	if err := t.feature.GetFeature(out[:]); err == nil {
		return out, nil
	}
	buf := make([]byte, reportSize+1)
	if err := t.feature.GetFeature(buf); err != nil {
		return out, fmt.Errorf("GET_FEATURE: %w", err)
	}
	copy(out[:], buf[1:])
	return out, nil
}

func (t *hidrawTransport) InitLCD() error {
	if t.lcd != nil {
		return nil
	}
	infos, err := hidraw.Enumerate()
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.VendorID != VendorID || info.ProductID != ProductID {
			continue
		}
		if info.Interface != lcdInterfaceID {
			continue
		}
		dev, err := hidraw.Open(info.Path)
		if err != nil {
			return fmt.Errorf("abriendo %s: %w", info.Path, err)
		}
		t.lcd = dev
		return nil
	}
	return fmt.Errorf("interfaz del LCD no encontrada")
}

func (t *hidrawTransport) WriteLCDPage(data []byte) error {
	if t.lcd == nil {
		return fmt.Errorf("LCD sin inicializar")
	}
	if _, err := t.lcd.Write(append([]byte{0x00}, data...)); err == nil {
		return nil
	}
	_, err := t.lcd.Write(data)
	return err
}

func (t *hidrawTransport) ReadLCDAck() ([]byte, error) {
	if t.lcd == nil {
		return nil, fmt.Errorf("LCD sin inicializar")
	}
	buf := make([]byte, 64)
	_, err := t.lcd.Read(buf, 300*time.Millisecond)
	return buf, err
}

func (t *hidrawTransport) Close() {
	if t.lcd != nil {
		_ = t.lcd.Close()
	}
	if t.feature != nil {
		_ = t.feature.Close()
	}
}
